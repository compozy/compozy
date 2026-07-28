import { useNavigate } from "@tanstack/react-router";
import { useSelector, useStore } from "@xstate/store-react";

import {
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
} from "@/systems/bridges";
import { getBridgeSetupProfile } from "@/systems/bridges/lib/bridge-setup";

import {
  createBridgeCreateFlowLogic,
  type BridgeCreateFlow,
  type BridgeSecretBindingExecution,
} from "./bridge-create-flow-store";

const bridgeCreateFlowLogic = createBridgeCreateFlowLogic();

interface BridgeCreateFlowInput {
  activeWorkspaceId?: string | null;
  activeWorkspaceName?: string | null;
  providers: BridgeProvider[];
}

export function useBridgeCreateFlow({
  activeWorkspaceId,
  activeWorkspaceName,
  providers,
}: BridgeCreateFlowInput) {
  const navigate = useNavigate({ from: "/bridges" });
  const createMutation = useCreateBridge();
  const putSecretBinding = usePutBridgeSecretBinding();
  const store = useStore(bridgeCreateFlowLogic);
  const flow = useSelector(store, snapshot => snapshot.context);

  const draft = hasDraft(flow) ? flow.draft : createBridgeCreateDraft(providers, activeWorkspaceId);
  const mode = hasDraft(flow) ? flow.mode : "simple";
  const committedBridge = flow.phase === "manifest" ? flow.bridge : null;
  const secretRecovery =
    flow.phase === "secret-recovery" || flow.phase === "retrying-secrets" ? flow.recovery : null;
  const selectedProvider =
    secretRecovery?.provider ?? findBridgeProviderByKey(providers, draft.selectedProviderKey);
  const supportsManifest =
    committedBridge !== null ||
    (selectedProvider !== undefined &&
      getBridgeSetupProfile(selectedProvider.platform)?.manifest === "slack");
  const manifestQuery = useSlackBridgeManifest(committedBridge?.id ?? "", {
    enabled: committedBridge !== null,
  });

  const navigateToBridge = async (bridge: { id: string }) => {
    await navigate({ params: { id: bridge.id }, to: "/bridges/$id" });
  };

  const bindSecrets: BridgeSecretBindingExecution = async (
    bridgeId,
    provider,
    submittedDraft,
    alreadyBound = [],
    expectedSlotNames
  ) => {
    const expectedNames = expectedSlotNames ? new Set(expectedSlotNames) : null;
    const slots = expectedNames
      ? provider.secret_slots?.filter(slot => expectedNames.has(slot.name))
      : provider.secret_slots;
    const filled = collectFilledBridgeSecretSlots(slots, submittedDraft.secretSlotValues);
    const filledNames = new Set(filled.map(slot => slot.name));
    const missing = expectedSlotNames?.filter(name => !filledNames.has(name)) ?? [];
    const outcome = await submitBridgeSecretSlots(bridgeId, filled, async ({ name, value }) => {
      const request = buildBridgeSecretBindingRequest(bridgeId, name, value, name);
      if (!request.ok) throw new Error(request.error);
      await putSecretBinding.mutateAsync({ bindingName: name, data: request.data, id: bridgeId });
    });

    return {
      bound: [...new Set([...alreadyBound, ...outcome.succeeded])],
      clearedSlotNames: filled.map(slot => slot.name),
      failures: {
        ...Object.fromEntries(
          missing.map(name => [name, "Enter a replacement value before retrying."])
        ),
        ...Object.fromEntries(outcome.failed.map(failure => [failure.name, failure.message])),
      },
    };
  };

  const submit = () => {
    if (flow.phase === "secret-recovery") {
      store.trigger.secretRetrySubmitted({ bindSecrets, navigate: navigateToBridge });
      return;
    }
    store.trigger.createSubmitted({
      activeWorkspaceId,
      bindSecrets,
      create: createMutation.mutateAsync,
      navigate: navigateToBridge,
      provider: findBridgeProviderByKey(providers, draft.selectedProviderKey),
    });
  };

  return {
    canCreateBridge: providers.some(isBridgeProviderSelectable),
    createDialogProps: {
      activeWorkspaceId,
      activeWorkspaceName,
      draft,
      isPending: flow.phase === "creating" || flow.phase === "retrying-secrets",
      manifestState: committedBridge
        ? {
            bridgeId: committedBridge.id,
            error: manifestQuery.error,
            isLoading: manifestQuery.isLoading,
            manifestJSON: manifestQuery.data
              ? JSON.stringify(manifestQuery.data.manifest, null, 2)
              : undefined,
            onOpenBridge: () => store.trigger.manifestOpened({ navigate: navigateToBridge }),
            onRetry: () => void manifestQuery.refetch(),
          }
        : null,
      mode,
      onDraftChange: (nextDraft: BridgeCreateDraft) =>
        store.trigger.draftChanged({ draft: nextDraft }),
      onModeChange: (nextMode: "advanced" | "simple") =>
        store.trigger.modeChanged({ mode: nextMode }),
      onOpenChange: (open: boolean) => {
        if (!open) store.trigger.dialogDismissed({ navigate: navigateToBridge });
      },
      onSubmit: submit,
      open: flow.phase !== "closed" && flow.phase !== "navigating",
      providers,
      secretRecovery,
      supportsManifest,
    },
    openCreateDialog: () =>
      store.trigger.createOpened({ draft: createBridgeCreateDraft(providers, activeWorkspaceId) }),
  };
}

function hasDraft(
  flow: BridgeCreateFlow
): flow is Extract<BridgeCreateFlow, { draft: BridgeCreateDraft }> {
  return "draft" in flow;
}
