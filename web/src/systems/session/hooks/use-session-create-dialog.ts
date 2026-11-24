import { useCallback, useContext, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { QueryClientContext } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";

import { toast, type EntityMode } from "@compozy/ui";

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
  type AutoPlacementCandidateView,
  type AutoResolutionSummary,
  type RunOnTarget,
} from "@/systems/computers";
import type {
  ComputerFixture,
  LocalComputerFixture,
} from "@/systems/computers/mocks/remote-computers-types";

import type { SessionCreateDialogDraft } from "../lib/session-create-draft";
import { ROOT_ENVIRONMENT_TARGET } from "../lib/session-environment-target";
import { restorePendingTerminalQuoteAfterFailedCreate } from "../lib/session-terminal-quote";
import { resolveSessionCreateDestination } from "../lib/session-create-destination";
import { sessionCreateStoreLogic, type SessionCreateStore } from "../stores/session-create-store";
import { useSessionEnvironment, type SessionEnvironmentModel } from "./use-session-environment";
import { type AgentPayload, useAgents } from "@/systems/agent";
import {
  useUserHomeDir,
  type DataScope,
  type DestinationKind,
  type SessionDestinationAxis,
  type WorkspacePayload,
} from "@/systems/workspace";
import { useSessionCreateSubmit } from "./use-session-create-submit";
import { useSessionCreateTarget } from "./use-session-create-target";
import { useSessionCreateDialogActions } from "./use-session-create-dialog-actions";

interface SessionCreateDialogContext {
  agents: AgentPayload[] | undefined;
  activeWorkspace: WorkspacePayload | undefined;
  dataScope?: DataScope | null;
  projectWorkspaceId?: string | null;
  /** The focused window's binding (`"<id>"` or `"@user"`): first in the project prefill. */
  focusedBinding?: string | null;
}

/** The inline pickers of the destination row; an axis is present only while aggregated. */
export interface SessionCreateDestinationAxes {
  project?: SessionDestinationAxis;
  profile?: SessionDestinationAxis;
}

export interface SessionCreateDialogState {
  open: boolean;
  restoreFocusOnClose: boolean;
  mode: EntityMode;
  agents: AgentPayload[];
  workspace: WorkspacePayload | undefined;
  workspaceId: string | null;
  scope: DestinationKind;
  destinationLabel: string;
  sessionRoot: string;
  destinationReady: boolean;
  sessionName: string;
  selectedAgentName: string;
  isSubmitting: boolean;
  submitError: string | null;
  pendingAgentName: string | null;
  pendingWorkspaceId: string | null;
  userHomeDir: string | undefined;
  /** Absent when the workspace is not git-backed. */
  environment: SessionEnvironmentModel["field"];
  environmentListingState: SessionEnvironmentModel["listingState"];
  environmentListingError?: string;
  /** True while a submit the operator already asked for waits on its worktree. */
  isAwaitingEnvironment: boolean;
  destinationAxes: SessionCreateDestinationAxes;
  computerId: RunOnTarget;
  computers: readonly ComputerFixture[];
  localComputer: LocalComputerFixture;
  loadBalancingEnabled: boolean;
  autoResolution: AutoResolutionSummary | null;
  autoLoading: boolean;
  autoCandidates: readonly AutoPlacementCandidateView[];
  autoNoEligible: boolean;
  autoNoEligibleReason: string | null;
  computerLocked: boolean;
  submitDisabled: boolean;
  offlineComputer: { id: string; name: string } | null;
  offlineComputerName: string | null;
  localExecutionBlocked: boolean;
  reconnectingComputerIds: readonly string[];
}

export interface SessionCreateDialogApi extends SessionCreateDialogState {
  onCancelEnvironment: () => void;
  onComputerChange: (next: RunOnTarget) => void;
  onReconnectComputer: (computerId: string) => void;
  onUpgradeComputer: (computerId: string) => void;
  onAddComputer: () => void;
  onOpenChange: (open: boolean) => void;
  onModeChange: (mode: EntityMode) => void;
  onAgentChange: (agentName: string) => void;
  onSessionNameChange: (next: string) => void;
  submit: () => void;
}

export interface SessionCreateDialogController {
  store: SessionCreateStore;
}

/** Provides the shared session-create store controller used by dialog hosts. */
export function useSessionCreateDialogController(): SessionCreateDialogController {
  return { store: useStore(sessionCreateStoreLogic) };
}

function resolveRemoteCreateAvailability(
  computerId: RunOnTarget,
  computers: ReturnType<typeof useComputers>["computers"]
): {
  isRemote: boolean;
  offlineComputer: { id: string; name: string } | null;
  submitDisabled: boolean;
} {
  const isRemote = isRemoteComputerId(computerId);
  const selectedComputer = isRemote ? findComputerById(computers, computerId) : undefined;
  const offlineComputer =
    isRemote && selectedComputer?.status === "offline"
      ? { id: selectedComputer.id, name: selectedComputer.name }
      : null;
  return {
    isRemote,
    offlineComputer,
    submitDisabled:
      isRemote &&
      selectedComputer !== undefined &&
      selectedComputer.status !== "connected" &&
      selectedComputer.status !== "needs_attention",
  };
}

function buildDestinationAxes(
  target: ReturnType<typeof useSessionCreateTarget>,
  store: SessionCreateStore
): SessionCreateDestinationAxes {
  const axes: SessionCreateDestinationAxes = {};
  if (target.projectAxis && target.projectValue !== null) {
    axes.project = {
      options: target.projectOptions,
      value: target.projectValue,
      onChange: (value: string) => {
        if (value === target.projectValue) return;
        target.selectProject(value);
        // Worktrees belong to one project: a new project starts at its root.
        store.trigger.environmentSelected({ environment: ROOT_ENVIRONMENT_TARGET });
      },
    };
  }
  if (target.profileAxis) {
    axes.profile = {
      options: target.profileOptions,
      value: target.profile,
      onChange: target.selectProfile,
    };
  }
  return axes;
}

/** Projects session-create state into the dialog API and validates durable launch requests. */
export function useSessionCreateDialogViewModel(
  {
    agents,
    activeWorkspace,
    dataScope,
    projectWorkspaceId,
    focusedBinding,
  }: SessionCreateDialogContext,
  store: SessionCreateStore
): SessionCreateDialogApi {
  const navigate = useNavigate();
  const queryClient = useContext(QueryClientContext);
  const userHomeDir = useUserHomeDir();
  const flow = useSelector(store, snapshot => snapshot.context);
  const { draft, pendingSubmit } = flow;
  const [lastConcreteComputerId, setLastConcreteComputerId] =
    useState<ComputerId>(LOCAL_COMPUTER_ID);
  const [reconnectingComputerIds, setReconnectingComputerIds] = useState<string[]>([]);

  const computers = useComputers({ enabled: flow.open, withHostResources: flow.open });
  const requestedComputerId = normalizeComputerId(draft.computerId) as RunOnTarget;
  const computerId = resolveEffectiveRunOnTarget({
    requested: requestedComputerId,
    local: computers.localView,
    computers: computers.computerViews,
    loadBalancingEnabled: computers.loadBalancingEnabled,
    allowAuto: true,
  });
  const isAuto = isAutoComputerId(computerId);
  const remoteState = resolveRemoteCreateAvailability(computerId, computers.computers);
  const pendingOp = pendingCreateOperation(flow.operation);

  // When Auto is selected, projects come from the last concrete computer (defaulting to local,
  // or the first connected remote if local has no focused workspace and local execution is off).
  const firstConnectedRemoteId = computers.computerViews.find(
    item => item.enabled && (item.status === "connected" || item.status === "needs_attention")
  )?.id;
  const projectSourceComputerId: ComputerId = isAuto
    ? isRemoteComputerId(lastConcreteComputerId)
      ? lastConcreteComputerId
      : !computers.localView.localRunAgents &&
          !activeWorkspace &&
          !projectWorkspaceId &&
          firstConnectedRemoteId
        ? firstConnectedRemoteId
        : LOCAL_COMPUTER_ID
    : computerId;

  // Each opening starts from the prefill; under All projects (or a remote computer) the picked
  // project (or User) is the destination, otherwise the project the opening named, else the
  // menubar destination.
  const target = useSessionCreateTarget({
    resetKey: flow.open,
    focusedBinding,
    seed: flow.target,
    computerId: projectSourceComputerId,
    open: flow.open,
  });
  const destinationInput =
    target.projectAxis || target.namedProject
      ? {
          activeWorkspace: target.workspace,
          projectWorkspaceId: target.workspace?.id ?? null,
          dataScope: target.scope,
          userHomeDir,
        }
      : { activeWorkspace, projectWorkspaceId, dataScope, userHomeDir };
  const destination = resolveSessionCreateDestination(destinationInput);
  // A named project the catalog has not confirmed blocks creation instead of falling back.
  const binding = target.namedProjectUnresolved ? null : destination.binding;

  const placement = useComputerPlacement({
    enabled: flow.open && isAuto && computers.loadBalancingEnabled,
    workspace: destination.scope === "project" ? destination.runtimeWorkspaceId : null,
    projectComputerId: projectSourceComputerId,
  });
  const autoCandidates =
    isAuto && placement.resolution
      ? buildAutoPlacementCandidates(
          placement.resolution.computerId,
          computers.localView,
          computers.computerViews
        )
      : [];
  const autoNoEligible = isAuto && placement.noEligible;
  const autoNoEligibleReason = autoNoEligible
    ? describeNoEligibleComputerReason(destination.destinationLabel, placement.reasons)
    : null;
  const localExecutionBlocked =
    computerId === LOCAL_COMPUTER_ID && !computers.localView.localRunAgents;

  const agentsComputerId =
    isAuto && placement.placement
      ? normalizeComputerId(placement.placement.computer_id)
      : projectSourceComputerId;
  const agentsWorkspaceId =
    isAuto && placement.placement
      ? placement.placement.workspace_id
      : destination.runtimeWorkspaceId;

  const agentList = useSessionCreateAgents({
    computerId: agentsComputerId,
    workspaceId: agentsWorkspaceId,
    destination,
    target,
    open: flow.open,
    agents,
  });
  const environment = useSessionEnvironment({
    workspaceId: destination.environmentWorkspaceId,
    target: draft.environment,
    onTargetChange: next => {
      restorePendingTerminalQuoteAfterFailedCreate(
        store.getSnapshot().context.pendingSubmit?.terminalQuote ?? null
      );
      store.trigger.environmentSelected({ environment: next });
    },
    enabled:
      flow.open &&
      !remoteState.isRemote &&
      !isAuto &&
      destination.scope === "project" &&
      binding !== null,
  });

  const submit = useSessionCreateSubmit({
    agents: agentList,
    binding,
    computerId,
    projectComputerId: projectSourceComputerId,
    environment,
    environmentWorkspaceId: destination.environmentWorkspaceId,
    runtimeWorkspaceId: destination.runtimeWorkspaceId,
    actingWorkspaceId: activeWorkspace?.id ?? null,
    scope: destination.scope,
    store,
    target,
  });
  const actions = useSessionCreateDialogActions({
    environment,
    runtimeWorkspaceId: destination.runtimeWorkspaceId,
    store,
  });

  const handleComputerChange = useCallback(
    (next: RunOnTarget) => {
      if (next !== "auto") {
        setLastConcreteComputerId(normalizeComputerId(next));
      }
      store.trigger.computerSelected({ computerId: next });
    },
    [store]
  );

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

  const handleUpgradeComputer = useCallback(() => {
    store.trigger.dialogOpenChanged({ open: false });
    void navigate({ to: "/settings/account" });
  }, [navigate, store]);

  const handleAddComputer = useCallback(() => {
    store.trigger.dialogOpenChanged({ open: false });
    void navigate({ to: "/settings/computers" });
  }, [navigate, store]);

  const submitDisabled =
    remoteState.submitDisabled ||
    localExecutionBlocked ||
    (isAuto && (placement.isLoading || placement.noEligible || !placement.resolution));

  return {
    open: flow.open,
    restoreFocusOnClose: flow.restoreFocusOnClose,
    mode: flow.mode,
    agents: agentList,
    workspace: activeWorkspace,
    workspaceId: destination.runtimeWorkspaceId,
    scope: destination.scope,
    destinationLabel: destination.destinationLabel,
    sessionRoot: destination.sessionRoot,
    destinationReady: binding !== null,
    sessionName: draft.sessionName,
    selectedAgentName: draft.agentName,
    isSubmitting: pendingOp.isSubmitting,
    submitError: flow.submitError,
    pendingAgentName: pendingOp.pendingAgentName,
    pendingWorkspaceId: pendingOp.pendingWorkspaceId,
    userHomeDir,
    environment: destination.scope === "project" && !isAuto ? environment.field : undefined,
    environmentListingState: isAuto ? "unsupported" : environment.listingState,
    environmentListingError: environment.listingError,
    isAwaitingEnvironment: pendingSubmit !== null,
    computerId,
    computers: computers.computerViews,
    localComputer: computers.localView,
    loadBalancingEnabled: computers.loadBalancingEnabled,
    autoResolution: isAuto ? placement.resolution : null,
    autoLoading: isAuto && placement.isLoading,
    autoCandidates,
    autoNoEligible,
    autoNoEligibleReason,
    computerLocked: draft.sent || pendingOp.isSubmitting,
    submitDisabled,
    offlineComputer: remoteState.offlineComputer,
    offlineComputerName: remoteState.offlineComputer?.name ?? null,
    localExecutionBlocked,
    reconnectingComputerIds,
    onComputerChange: handleComputerChange,
    onReconnectComputer: handleReconnectComputer,
    onUpgradeComputer: handleUpgradeComputer,
    onAddComputer: handleAddComputer,
    destinationAxes: buildDestinationAxes(target, store),
    ...actions,
    submit,
  };
}

export type { SessionCreateDialogDraft };

function pendingCreateOperation(operation: {
  status: string;
  agentName?: string;
  workspaceId?: string | null;
}) {
  if (operation.status !== "submitting") {
    return { isSubmitting: false, pendingAgentName: null, pendingWorkspaceId: null };
  }
  return {
    isSubmitting: true,
    pendingAgentName: operation.agentName ?? null,
    pendingWorkspaceId: operation.workspaceId ?? null,
  };
}

/**
 * The agents a new session can run: user-layer agents for User, the project's otherwise, read in
 * the picked profile. The shell's list is reused only when no picker changes the destination.
 */
function useSessionCreateAgents({
  computerId,
  workspaceId,
  destination,
  target,
  open,
  agents,
}: {
  computerId: ComputerId;
  workspaceId: string | null;
  destination: ReturnType<typeof resolveSessionCreateDestination>;
  target: ReturnType<typeof useSessionCreateTarget>;
  open: boolean;
  agents: SessionCreateDialogContext["agents"];
}) {
  const userTarget = destination.scope === "user";
  const query = useAgents(userTarget ? null : (workspaceId ?? ""), {
    enabled: userTarget ? open : Boolean(workspaceId),
    profile: target.profile,
    computerId,
  });
  if (query.data) return query.data;
  // The shell's agents belong to its own project: never stand in for another target's.
  return !isRemoteComputerId(computerId) &&
    workspaceId &&
    !target.projectAxis &&
    !target.profileAxis &&
    !target.namedProject &&
    !target.namedProjectUnresolved
    ? (agents ?? [])
    : [];
}
