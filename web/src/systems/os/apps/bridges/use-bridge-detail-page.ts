import { useDeferredValue, useState } from "react";
import { toast } from "sonner";

import type { EntityMode } from "@agh/ui";

import {
  buildBridgeUpdateRequest,
  bridgeListFilterForScope,
  createBridgeUpdateDraft,
  fingerprintBridgeProviderConfig,
  isBridgeUpdateDraftDirty,
  useBridge,
  useBridgeHealthStream,
  useBridgeProviders,
  useBridgeRoutes,
  useBridgeSecretBindings,
  useBridgeTargets,
  useBridges,
  useDisableBridge,
  useEnableBridge,
  useRestartBridge,
  useResolveBridgeTarget,
  useUpdateBridge,
} from "@/systems/bridges";
import type { BridgeResolveTargetResponse, BridgeUpdateDraft } from "@/systems/bridges";
import { useActiveWorkspace } from "@/systems/workspace";
import { useBridgeDeliveryTests } from "@/hooks/routes/use-bridge-delivery-tests";
import { useBridgeSecretRotations } from "./use-bridge-secret-rotations";
import { useBridgeSetupFlow } from "@/hooks/routes/use-bridge-setup-flow";

function useBridgeDetailPage(bridgeId: string) {
  const { activeWorkspace, activeWorkspaceId } = useActiveWorkspace();

  const [isEditDialogOpen, setEditDialogOpen] = useState(false);
  const [editMode, setEditMode] = useState<EntityMode>("simple");
  const [editDraft, setEditDraft] = useState<BridgeUpdateDraft>(() => createBridgeUpdateDraft());
  // The draft as loaded, so the PATCH gate can require a real change.
  const [pristineEditDraft, setPristineEditDraft] = useState<BridgeUpdateDraft>(() =>
    createBridgeUpdateDraft()
  );
  const [restartRequiredByID, setRestartRequiredByID] = useState<Record<string, true>>({});
  const [targetSearchQuery, setTargetSearchQuery] = useState("");
  const [targetResolveInput, setTargetResolveInput] = useState("");
  const [targetResolveResult, setTargetResolveResult] =
    useState<BridgeResolveTargetResponse | null>(null);

  const deferredTargetSearchQuery = useDeferredValue(targetSearchQuery);

  const bridgeListFilters = bridgeListFilterForScope("all", activeWorkspaceId);

  const bridgesQuery = useBridges(bridgeListFilters, { enabled: Boolean(bridgeId) });
  useBridgeHealthStream({
    bridgeIds: bridgeId ? [bridgeId] : [],
    enabled: Boolean(bridgeId),
    filters: bridgeListFilters,
  });
  const providersQuery = useBridgeProviders();
  const updateBridgeMutation = useUpdateBridge();
  const enableBridgeMutation = useEnableBridge();
  const disableBridgeMutation = useDisableBridge();
  const restartBridgeMutation = useRestartBridge();
  const resolveBridgeTargetMutation = useResolveBridgeTarget();

  const bridges = bridgesQuery.bridges;
  const bridgeHealth = bridgesQuery.bridgeHealth;
  const providers = providersQuery.data ?? [];

  const listBridgeSummary = bridges.find(bridge => bridge.id === bridgeId);

  const bridgeDetailQuery = useBridge(bridgeId, { enabled: Boolean(bridgeId) });
  const bridgeRoutesQuery = useBridgeRoutes(bridgeId, { enabled: Boolean(bridgeId) });
  const bridgeTargetsQuery = useBridgeTargets(
    bridgeId,
    { limit: 50, q: deferredTargetSearchQuery },
    { enabled: Boolean(bridgeId) }
  );
  const bridgeSecretBindingsQuery = useBridgeSecretBindings(bridgeId, {
    enabled: Boolean(bridgeId),
  });

  // Prefer the detail record; fall back to the list summary while it loads.
  const selectedBridge = bridgeDetailQuery.data?.bridge ?? listBridgeSummary;
  const selectedBridgeProvider = selectedBridge
    ? providers.find(
        provider =>
          provider.extension_name === selectedBridge.extension_name &&
          provider.platform === selectedBridge.platform
      )
    : undefined;
  const selectedHealth =
    bridgeDetailQuery.data?.health ?? (bridgeId ? bridgeHealth[bridgeId] : undefined);
  const selectedSecretBindings = bridgeSecretBindingsQuery.data ?? [];
  const setupFactsReady =
    providersQuery.data !== undefined && bridgeSecretBindingsQuery.data !== undefined;
  const deliveryTests = useBridgeDeliveryTests(selectedBridge);
  const setupFlow = useBridgeSetupFlow({
    bindings: selectedSecretBindings,
    bridge: setupFactsReady ? selectedBridge : undefined,
    health: selectedHealth,
    provider: selectedBridgeProvider,
  });
  const secrets = useBridgeSecretRotations({
    bindings: selectedSecretBindings,
    bridge: selectedBridge,
    onCredentialChanged: bridgeId => {
      setupFlow.clearEvidence();
      markRestartRequired(bridgeId);
    },
    provider: selectedBridgeProvider,
  });
  const restartRequired =
    selectedBridge != null ? Boolean(restartRequiredByID[selectedBridge.id]) : false;
  const isLifecyclePending =
    enableBridgeMutation.isPending ||
    disableBridgeMutation.isPending ||
    restartBridgeMutation.isPending;
  const isSecretBindingPending = secrets.isPending;

  // Only the primary detail query is fatal; route/secret failures stay sectional.
  const detailError = bridgeDetailQuery.error ?? null;
  const detailLoading =
    Boolean(bridgeId) &&
    bridgeDetailQuery.isLoading &&
    !bridgeDetailQuery.data &&
    !listBridgeSummary;

  const markRestartRequired = (id: string) => {
    setRestartRequiredByID(current => ({
      ...current,
      [id]: true,
    }));
  };

  const clearRestartRequired = (id: string) => {
    setRestartRequiredByID(current => {
      if (!(id in current)) {
        return current;
      }

      const next = { ...current };
      delete next[id];
      return next;
    });
  };

  const openEditDialog = (mode: EntityMode = "simple") => {
    if (!selectedBridge) {
      return;
    }

    const loaded = createBridgeUpdateDraft(selectedBridge);
    setEditDraft(loaded);
    setPristineEditDraft(loaded);
    setEditMode(mode);
    deliveryTests.resetDraft();
    setEditDialogOpen(true);
  };

  /** D2: the delivery test is reachable only through the editor's Advanced tier. */
  const openDeliveryTest = () => openEditDialog("advanced");

  const handleEditDialogOpenChange = (open: boolean) => {
    setEditDialogOpen(open);
  };

  /**
   * One primary action commits both halves of an edit: the PATCH for mutable
   * fields and a binding PUT per rotated slot. Secrets cannot ride the PATCH —
   * `UpdateBridgeRequest` has no secret field — so they are written separately
   * but never behind a second, competing button.
   */
  const handleUpdateBridge = async () => {
    if (!selectedBridge) {
      return;
    }

    const requestResult = buildBridgeUpdateRequest(editDraft);
    if (!requestResult.ok) {
      toast.error(requestResult.error);
      return;
    }

    if (!(await secrets.commitPending())) {
      return;
    }

    if (!isBridgeUpdateDraftDirty(editDraft, pristineEditDraft)) {
      setEditDialogOpen(false);
      return;
    }

    try {
      const result = await updateBridgeMutation.mutateAsync({
        data: requestResult.data,
        id: selectedBridge.id,
      });

      setEditDialogOpen(false);
      const providerConfigChanged =
        fingerprintBridgeProviderConfig(selectedBridge.provider_config) !==
        fingerprintBridgeProviderConfig(result.bridge.provider_config);
      if (providerConfigChanged) {
        setupFlow.clearEvidence();
      } else {
        setupFlow.clearVerification();
      }
      markRestartRequired(selectedBridge.id);
      toast.success(`Updated bridge ${result.bridge.display_name}. Restart to apply changes.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to update bridge");
    }
  };

  const handleEnableBridge = async () => {
    if (!selectedBridge) {
      return;
    }

    setupFlow.clearVerification();
    try {
      const result = await enableBridgeMutation.mutateAsync({ id: selectedBridge.id });
      clearRestartRequired(result.bridge.id);
      toast.success(`Enabled bridge ${result.bridge.display_name}.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to enable bridge");
    }
  };

  const handleDisableBridge = async () => {
    if (!selectedBridge) {
      return;
    }

    setupFlow.clearVerification();
    try {
      const result = await disableBridgeMutation.mutateAsync({ id: selectedBridge.id });
      toast.success(`Disabled bridge ${result.bridge.display_name}.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to disable bridge");
    }
  };

  const handleRestartBridge = async () => {
    if (!selectedBridge) {
      return;
    }

    setupFlow.clearVerification();
    try {
      const result = await restartBridgeMutation.mutateAsync({ id: selectedBridge.id });
      clearRestartRequired(result.bridge.id);
      toast.success(`Restarted bridge ${result.bridge.display_name}.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to restart bridge");
    }
  };

  const handleTargetSearchChange = (query: string) => {
    setTargetSearchQuery(query);
  };

  const handleTargetResolveInputChange = (value: string) => {
    setTargetResolveInput(value);
    setTargetResolveResult(null);
  };

  const handleResolveBridgeTarget = async () => {
    if (!selectedBridge) {
      return;
    }

    const name = targetResolveInput.trim();
    if (name === "") {
      toast.error("Enter a bridge target name to resolve.");
      return;
    }

    try {
      const result = await resolveBridgeTargetMutation.mutateAsync({
        id: selectedBridge.id,
        data: { name },
      });
      setTargetResolveResult(result);
      if (result.result.match) {
        toast.success(`Resolved target ${result.result.match.display_name}.`);
        return;
      }
      if (result.result.ambiguous) {
        toast.error(result.diagnostic?.message ?? "Bridge target matched multiple candidates.");
        return;
      }
      toast.error(result.diagnostic?.message ?? "Bridge target was not found.");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to resolve bridge target");
    }
  };

  const detailPanelProps = {
    bridge: selectedBridge,
    emptyMessage: "Bridge not found.",
    error: detailError,
    health: selectedHealth,
    state: {
      isLifecyclePending,
      isLoading: detailLoading,
      isProviderLoading: providersQuery.isLoading && !providersQuery.data,
      isRoutesLoading: bridgeRoutesQuery.isLoading && !bridgeRoutesQuery.data,
      isSecretBindingPending,
      isSecretBindingsLoading:
        bridgeSecretBindingsQuery.isLoading && !bridgeSecretBindingsQuery.data,
      providerError: providersQuery.error,
      secretBindingsError: bridgeSecretBindingsQuery.error,
    },
    targetDirectory: {
      error: bridgeTargetsQuery.error,
      isLoading: bridgeTargetsQuery.isLoading && !bridgeTargetsQuery.data,
      isResolving: resolveBridgeTargetMutation.isPending,
      onQueryChange: handleTargetSearchChange,
      onResolveInputChange: handleTargetResolveInputChange,
      onResolveSubmit: handleResolveBridgeTarget,
      query: targetSearchQuery,
      resolveInput: targetResolveInput,
      resolveResult: targetResolveResult,
      response: bridgeTargetsQuery.data,
    },
    onDeleteSecretBinding: secrets.remove,
    onDisableBridge: handleDisableBridge,
    onEnableBridge: handleEnableBridge,
    onOpenEdit: () => openEditDialog("simple"),
    onOpenDeliveryTest: openDeliveryTest,
    onRestartBridge: handleRestartBridge,
    onSaveSecretBinding: secrets.save,
    onSecretDraftChange: secrets.setDraft,
    provider: selectedBridgeProvider,
    restartRequired,
    routes: bridgeRoutesQuery.data ?? [],
    secretBindings: selectedSecretBindings,
    secretInputValues: secrets.draftsByName,
    setup: {
      isLifecyclePending,
      isRegistering: setupFlow.isRegistering,
      isVerifying: setupFlow.isVerifying,
      onRegisterWebhook: setupFlow.registerWebhook,
      onVerify: setupFlow.verify,
      projection: setupFlow.projection,
    },
    workspaceName:
      selectedBridge?.scope === "workspace" && selectedBridge.workspace_id === activeWorkspaceId
        ? activeWorkspace?.name
        : selectedBridge?.workspace_id,
  };

  const editDialogProps = {
    allowProviderDefaultDmPolicy: selectedBridge?.dm_policy == null,
    bridgeName: selectedBridge?.display_name,
    deliveryTest: deliveryTests.panelProps,
    draft: editDraft,
    hasPendingSecretRotation: secrets.hasPending,
    identity: selectedBridge
      ? {
          extensionName: selectedBridge.extension_name,
          platform: selectedBridge.platform,
          scope: selectedBridge.scope,
          workspaceId: selectedBridge.workspace_id ?? null,
        }
      : undefined,
    isPending: updateBridgeMutation.isPending || secrets.isPending,
    mode: editMode,
    onDraftChange: setEditDraft,
    onModeChange: setEditMode,
    onOpenChange: handleEditDialogOpenChange,
    onSubmit: handleUpdateBridge,
    open: isEditDialogOpen,
    pristineDraft: pristineEditDraft,
    provider: selectedBridgeProvider,
    rotation: {
      kind: "managed" as const,
      onRotationEditingChange: secrets.setEditing,
      onRotationValueChange: secrets.setDraft,
      rotations: secrets.rotations,
    },
    statusLabel: selectedBridge
      ? `${selectedBridge.enabled ? "enabled" : "disabled"} · ${selectedBridge.platform}`
      : undefined,
  };

  return {
    detailPanelProps,
    editDialogProps,
    selectedBridge,
  };
}

export { useBridgeDetailPage };
