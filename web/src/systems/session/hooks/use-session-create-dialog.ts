import { useSelector, useStore } from "@xstate/store-react";

import type { EntityMode } from "@compozy/ui";

import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import type {
  AutoPlacementCandidateView,
  AutoResolutionSummary,
  RunOnTarget,
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
import {
  useSessionCreateComputerActions,
  useSessionCreateComputerTarget,
  useSessionCreatePlacement,
} from "./use-session-create-computer";
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

function resolveDialogDestination(
  target: ReturnType<typeof useSessionCreateTarget>,
  local: Parameters<typeof resolveSessionCreateDestination>[0]
) {
  const destination = resolveSessionCreateDestination(
    target.projectAxis || target.namedProject
      ? {
          activeWorkspace: target.workspace,
          projectWorkspaceId: target.workspace?.id ?? null,
          dataScope: target.scope,
          userHomeDir: local.userHomeDir,
        }
      : local
  );
  // A named project the catalog has not confirmed blocks creation instead of falling back.
  const binding = target.namedProjectUnresolved ? null : destination.binding;
  return { destination, binding };
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
  const userHomeDir = useUserHomeDir();
  const flow = useSelector(store, snapshot => snapshot.context);
  const { draft, pendingSubmit } = flow;
  const pendingOp = pendingCreateOperation(flow.operation);

  const {
    computerId,
    computers,
    isAuto,
    localExecutionBlocked,
    onComputerChange,
    projectSourceComputerId,
    remoteState,
  } = useSessionCreateComputerTarget({
    hasLocalProject: Boolean(activeWorkspace || projectWorkspaceId),
    open: flow.open,
    requested: draft.computerId,
    store,
  });

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
  const { destination, binding } = resolveDialogDestination(target, {
    activeWorkspace,
    projectWorkspaceId,
    dataScope,
    userHomeDir,
  });

  const placement = useSessionCreatePlacement({
    isAuto,
    open: flow.open,
    computers,
    projectComputerId: projectSourceComputerId,
    destination,
  });
  const agentList = useSessionCreateAgents({
    computerId: placement.agentsComputerId,
    workspaceId: placement.agentsWorkspaceId,
    destination,
    target,
    open: flow.open,
    agents,
  });
  const showEnvironment = destination.scope === "project" && !isAuto;
  const environment = useSessionEnvironment({
    workspaceId: destination.environmentWorkspaceId,
    target: draft.environment,
    onTargetChange: next => {
      restorePendingTerminalQuoteAfterFailedCreate(
        store.getSnapshot().context.pendingSubmit?.terminalQuote ?? null
      );
      store.trigger.environmentSelected({ environment: next });
    },
    enabled: flow.open && !remoteState.isRemote && showEnvironment && binding !== null,
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

  const computerActions = useSessionCreateComputerActions(store, actions.onOpenChange);

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
    environment: showEnvironment ? environment.field : undefined,
    environmentListingState: isAuto ? "unsupported" : environment.listingState,
    environmentListingError: environment.listingError,
    isAwaitingEnvironment: pendingSubmit !== null,
    computerId,
    computers: computers.computerViews,
    localComputer: computers.localView,
    loadBalancingEnabled: computers.loadBalancingEnabled,
    autoResolution: placement.autoResolution,
    autoLoading: placement.autoLoading,
    autoCandidates: placement.autoCandidates,
    autoNoEligible: placement.autoNoEligible,
    autoNoEligibleReason: placement.autoNoEligibleReason,
    computerLocked: draft.sent || pendingOp.isSubmitting,
    submitDisabled: remoteState.submitDisabled || localExecutionBlocked || placement.submitBlocked,
    offlineComputer: remoteState.offlineComputer,
    offlineComputerName: remoteState.offlineComputer?.name ?? null,
    localExecutionBlocked,
    onComputerChange,
    destinationAxes: buildDestinationAxes(target, store),
    ...actions,
    ...computerActions,
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
