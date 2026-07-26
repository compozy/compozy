import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";

import type { EntityMode } from "@agh/ui";

import {
  buildBridgeCreateRequest,
  buildBridgeSecretBindingRequest,
  collectFilledBridgeSecretSlots,
  createBridgeCreateDraft,
  findBridgeProviderByKey,
  isBridgeProviderSelectable,
  submitBridgeSecretSlots,
  useCreateBridge,
  usePutBridgeSecretBinding,
  useSlackBridgeManifest,
  type BridgeCreateDraft,
  type BridgeProvider,
  type BridgeSecretRecoveryState,
} from "@/systems/bridges";
import { getBridgeSetupProfile } from "@/systems/bridges/lib/bridge-setup";

interface BridgeCreateFlowInput {
  activeWorkspaceId?: string | null;
  activeWorkspaceName?: string | null;
  providers: BridgeProvider[];
}

interface PersistedBridge {
  displayName: string;
  id: string;
}

interface CommittedBridge extends PersistedBridge {
  manifest: "slack";
}

export function useBridgeCreateFlow({
  activeWorkspaceId,
  activeWorkspaceName,
  providers,
}: BridgeCreateFlowInput) {
  const navigate = useNavigate({ from: "/bridges" });
  const [isOpen, setOpen] = useState(false);
  const [mode, setMode] = useState<EntityMode>("simple");
  const [draft, setDraft] = useState<BridgeCreateDraft>(() =>
    createBridgeCreateDraft([], activeWorkspaceId)
  );
  const [committedBridge, setCommittedBridge] = useState<CommittedBridge | null>(null);
  const [secretRecovery, setSecretRecovery] = useState<BridgeSecretRecoveryState | null>(null);
  const createMutation = useCreateBridge();
  const putSecretBinding = usePutBridgeSecretBinding();

  const selectedProvider = findBridgeProviderByKey(providers, draft.selectedProviderKey);
  const supportsManifest =
    committedBridge?.manifest === "slack" ||
    (selectedProvider !== undefined &&
      getBridgeSetupProfile(selectedProvider.platform)?.manifest === "slack");
  const manifestQuery = useSlackBridgeManifest(committedBridge?.id ?? "", {
    enabled: isOpen && committedBridge !== null,
  });

  const navigateToBridge = (bridge: PersistedBridge) => {
    setOpen(false);
    setCommittedBridge(null);
    setSecretRecovery(null);
    void navigate({ params: { id: bridge.id }, to: "/bridges/$id" });
  };

  const openCreateDialog = () => {
    setCommittedBridge(null);
    setSecretRecovery(null);
    setMode("simple");
    setDraft(createBridgeCreateDraft(providers, activeWorkspaceId));
    setOpen(true);
  };

  const handleOpenChange = (open: boolean) => {
    if (!open && secretRecovery) {
      navigateToBridge({ displayName: draft.displayName, id: secretRecovery.bridgeId });
      return;
    }
    if (!open && committedBridge) {
      navigateToBridge(committedBridge);
      return;
    }
    setOpen(open);
  };

  /**
   * Phase two: bind every filled slot to the bridge that phase one created.
   * The plaintext leaves the draft either way — a failed write reopens the slot
   * empty rather than echoing what was typed.
   */
  const bindSecretSlots = async (
    bridgeId: string,
    provider: BridgeProvider,
    alreadyBound: string[] = [],
    expectedSlotNames?: readonly string[]
  ) => {
    const expectedSlotNameSet = expectedSlotNames ? new Set(expectedSlotNames) : null;
    const slots = expectedSlotNameSet
      ? provider.secret_slots?.filter(slot => expectedSlotNameSet.has(slot.name))
      : provider.secret_slots;
    const filled = collectFilledBridgeSecretSlots(slots, draft.secretSlotValues);
    const missingSlotNames = expectedSlotNames?.filter(
      name => !filled.some(slot => slot.name === name)
    );
    if (missingSlotNames?.length) {
      return {
        bound: alreadyBound,
        failures: Object.fromEntries(
          missingSlotNames.map(name => [name, "Enter a replacement value before retrying."])
        ),
      };
    }
    if (filled.length === 0) return { bound: alreadyBound, failures: {} as Record<string, string> };

    const outcome = await submitBridgeSecretSlots(bridgeId, filled, async ({ name, value }) => {
      const request = buildBridgeSecretBindingRequest(bridgeId, name, value, name);
      if (!request.ok) throw new Error(request.error);
      await putSecretBinding.mutateAsync({ bindingName: name, data: request.data, id: bridgeId });
    });

    const failures = Object.fromEntries(
      outcome.failed.map(failure => [failure.name, failure.message])
    );
    setDraft(current => {
      const secretSlotValues = { ...current.secretSlotValues };
      for (const slot of filled) secretSlotValues[slot.name] = "";
      return { ...current, secretSlotValues };
    });

    return { bound: [...alreadyBound, ...outcome.succeeded], failures };
  };

  const retrySecretBinding = async () => {
    if (!secretRecovery) return;
    const result = await bindSecretSlots(
      secretRecovery.bridgeId,
      secretRecovery.provider,
      secretRecovery.bound,
      Object.keys(secretRecovery.failures)
    );
    if (Object.keys(result.failures).length === 0) {
      toast.success("Bound the remaining bridge credentials.");
      navigateToBridge({ displayName: draft.displayName, id: secretRecovery.bridgeId });
      return;
    }
    setSecretRecovery({
      bridgeId: secretRecovery.bridgeId,
      bound: result.bound,
      failures: result.failures,
      provider: secretRecovery.provider,
    });
  };

  const submit = async () => {
    if (secretRecovery) {
      await retrySecretBinding();
      return;
    }

    const provider = findBridgeProviderByKey(providers, draft.selectedProviderKey);
    if (!provider || !isBridgeProviderSelectable(provider)) {
      toast.error("Select an available bridge provider before creating the bridge.");
      return;
    }

    if (draft.scope === "workspace" && !activeWorkspaceId) {
      toast.error("Select an active workspace before creating a workspace-scoped bridge.");
      return;
    }

    const requestResult = buildBridgeCreateRequest(draft, provider, activeWorkspaceId);
    if (!requestResult.ok) {
      toast.error(requestResult.error);
      return;
    }

    try {
      const result = await createMutation.mutateAsync(requestResult.data);
      const committed = {
        displayName: result.bridge.display_name,
        id: result.bridge.id,
      };

      const secrets = await bindSecretSlots(committed.id, provider);
      if (Object.keys(secrets.failures).length > 0) {
        setSecretRecovery({
          bridgeId: committed.id,
          bound: secrets.bound,
          failures: secrets.failures,
          provider,
        });
        toast.error(
          `Created bridge ${committed.displayName}, but some credentials failed to bind.`
        );
        return;
      }

      if (getBridgeSetupProfile(provider.platform)?.manifest === "slack") {
        setCommittedBridge({ ...committed, manifest: "slack" });
        toast.success(`Created bridge ${committed.displayName}. Continue with its Slack manifest.`);
        return;
      }

      toast.success(`Created bridge ${committed.displayName}.`);
      navigateToBridge(committed);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to create bridge");
    }
  };

  return {
    canCreateBridge: providers.some(isBridgeProviderSelectable),
    createDialogProps: {
      activeWorkspaceId,
      activeWorkspaceName,
      draft,
      isPending: createMutation.isPending || putSecretBinding.isPending,
      manifestState: committedBridge
        ? {
            bridgeId: committedBridge.id,
            error: manifestQuery.error,
            isLoading: manifestQuery.isLoading,
            manifestJSON: manifestQuery.data
              ? JSON.stringify(manifestQuery.data.manifest, null, 2)
              : undefined,
            onOpenBridge: () => navigateToBridge(committedBridge),
            onRetry: () => void manifestQuery.refetch(),
          }
        : null,
      mode,
      onDraftChange: setDraft,
      onModeChange: setMode,
      onOpenChange: handleOpenChange,
      onSubmit: submit,
      open: isOpen,
      providers,
      secretRecovery,
      supportsManifest,
    },
    openCreateDialog,
  };
}
