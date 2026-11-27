import { useCallback, useContext, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { QueryClientContext } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import { useOptionalUpgradeSheetActions } from "@/systems/account";
import {
  isAutoComputerId,
  isRemoteComputerId,
  LOCAL_COMPUTER_ID,
  normalizeComputerId,
  type ComputerId,
} from "@/lib/computer-scope";
import {
  buildAutoPlacementCandidates,
  computerRegistryKeys,
  describeNoEligibleComputerReason,
  findComputerById,
  patchComputer,
  resolveEffectiveRunOnTarget,
  useComputerPlacement,
  useComputers,
  type RunOnTarget,
} from "@/systems/computers";

import type { resolveSessionCreateDestination } from "../lib/session-create-destination";
import type { SessionCreateStore } from "../stores/session-create-store";

type ComputerCatalog = ReturnType<typeof useComputers>;
type SessionDestination = ReturnType<typeof resolveSessionCreateDestination>;

function sessionProjectComputer({
  computerId,
  lastConcreteComputerId,
  hasLocalProject,
  computers,
}: {
  computerId: ComputerId;
  lastConcreteComputerId: ComputerId;
  hasLocalProject: boolean;
  computers: ComputerCatalog;
}): ComputerId {
  if (!isAutoComputerId(computerId)) return computerId;
  if (isRemoteComputerId(lastConcreteComputerId)) return lastConcreteComputerId;
  if (computers.localView.localRunAgents || hasLocalProject) return LOCAL_COMPUTER_ID;
  const firstConnectedRemote = computers.computerViews.find(
    item => item.enabled && (item.status === "connected" || item.status === "needs_attention")
  );
  return firstConnectedRemote?.id ?? LOCAL_COMPUTER_ID;
}

function resolveRemoteCreateAvailability(
  computerId: RunOnTarget,
  computers: ComputerCatalog["computers"]
) {
  const isRemote = isRemoteComputerId(computerId);
  const selectedComputer = isRemote ? findComputerById(computers, computerId) : undefined;
  const offlineComputer =
    selectedComputer?.status === "offline"
      ? { id: selectedComputer.id, name: selectedComputer.name }
      : null;
  return {
    isRemote,
    offlineComputer,
    submitDisabled:
      selectedComputer !== undefined &&
      selectedComputer.status !== "connected" &&
      selectedComputer.status !== "needs_attention",
  };
}

export function useSessionCreateComputerTarget({
  hasLocalProject,
  open,
  requested,
  store,
}: {
  hasLocalProject: boolean;
  open: boolean;
  requested: ComputerId;
  store: SessionCreateStore;
}) {
  const [lastConcreteComputerId, setLastConcreteComputerId] =
    useState<ComputerId>(LOCAL_COMPUTER_ID);
  const computers = useComputers({ enabled: open, withHostResources: open });
  const computerId = resolveEffectiveRunOnTarget({
    requested: normalizeComputerId(requested),
    local: computers.localView,
    computers: computers.computerViews,
    loadBalancingEnabled: computers.loadBalancingEnabled,
    allowAuto: true,
  });
  const projectSourceComputerId = sessionProjectComputer({
    computerId,
    lastConcreteComputerId,
    hasLocalProject,
    computers,
  });
  const onComputerChange = useCallback(
    (next: RunOnTarget) => {
      if (next !== "auto") setLastConcreteComputerId(normalizeComputerId(next));
      store.trigger.computerSelected({ computerId: next });
    },
    [store]
  );
  return {
    computerId,
    computers,
    isAuto: isAutoComputerId(computerId),
    localExecutionBlocked: computerId === LOCAL_COMPUTER_ID && !computers.localView.localRunAgents,
    onComputerChange,
    projectSourceComputerId,
    remoteState: resolveRemoteCreateAvailability(computerId, computers.computers),
  };
}

function placementView(
  isAuto: boolean,
  placement: ReturnType<typeof useComputerPlacement>,
  computers: ComputerCatalog,
  destination: SessionDestination,
  projectComputerId: ComputerId
) {
  const resolution = isAuto ? placement.resolution : null;
  const winner = isAuto ? placement.placement : null;
  const autoNoEligible = isAuto && placement.noEligible;
  return {
    autoResolution: resolution,
    autoLoading: isAuto && placement.isLoading,
    autoCandidates: resolution
      ? buildAutoPlacementCandidates(
          resolution.computerId,
          computers.localView,
          computers.computerViews
        )
      : [],
    autoNoEligible,
    autoNoEligibleReason: autoNoEligible
      ? describeNoEligibleComputerReason(destination.destinationLabel, placement.reasons)
      : null,
    agentsComputerId: winner ? normalizeComputerId(winner.computer_id) : projectComputerId,
    agentsWorkspaceId: winner ? winner.workspace_id : destination.runtimeWorkspaceId,
    submitBlocked: isAuto && (placement.isLoading || placement.noEligible || !resolution),
  };
}

export function useSessionCreatePlacement({
  isAuto,
  open,
  computers,
  projectComputerId,
  destination,
}: {
  isAuto: boolean;
  open: boolean;
  computers: ComputerCatalog;
  projectComputerId: ComputerId;
  destination: SessionDestination;
}) {
  const placement = useComputerPlacement({
    enabled: open && isAuto && computers.loadBalancingEnabled,
    workspace: destination.scope === "project" ? destination.runtimeWorkspaceId : null,
    projectComputerId,
  });
  return placementView(isAuto, placement, computers, destination, projectComputerId);
}

export function useSessionCreateComputerActions(
  store: SessionCreateStore,
  onOpenChange: (open: boolean) => void
) {
  const navigate = useNavigate();
  const queryClient = useContext(QueryClientContext);
  const upgradeSheet = useOptionalUpgradeSheetActions();
  const [reconnectingComputerIds, setReconnectingComputerIds] = useState<string[]>([]);

  const handleReconnectComputer = useCallback(
    (targetId: string) => {
      setReconnectingComputerIds(prev => (prev.includes(targetId) ? prev : [...prev, targetId]));
      void patchComputer(targetId, { enabled: true })
        .then(() => {
          void queryClient?.invalidateQueries({ queryKey: computerRegistryKeys.all });
        })
        .catch(err => {
          toast.error(err instanceof Error ? err.message : "Failed to reconnect computer.");
        })
        .finally(() => {
          setReconnectingComputerIds(prev => prev.filter(item => item !== targetId));
        });
    },
    [queryClient]
  );

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (!nextOpen && upgradeSheet?.open) return;
      onOpenChange(nextOpen);
    },
    [onOpenChange, upgradeSheet?.open]
  );

  const handleUpgradeComputer = useCallback(() => {
    if (upgradeSheet) {
      upgradeSheet.openUpgradeSheet();
      return;
    }
    store.trigger.dialogOpenChanged({ open: false });
    void navigate({ to: "/settings/account" });
  }, [navigate, store, upgradeSheet]);

  const handleAddComputer = useCallback(() => {
    store.trigger.dialogOpenChanged({ open: false });
    void navigate({ to: "/settings/computers" });
  }, [navigate, store]);

  return {
    reconnectingComputerIds,
    onReconnectComputer: handleReconnectComputer,
    onUpgradeComputer: handleUpgradeComputer,
    onAddComputer: handleAddComputer,
    onOpenChange: handleOpenChange,
  };
}
