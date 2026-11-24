import { useSelector, useStore } from "@xstate/store-react";

import type { EntityMode } from "@compozy/ui";

import { isRemoteComputerId, normalizeComputerId, type ComputerId } from "@/lib/computer-scope";
import { findComputerById, useComputers, type RunOnTarget } from "@/systems/computers";
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
  computerId: ComputerId;
  computers: readonly ComputerFixture[];
  localComputer: LocalComputerFixture;
  computerLocked: boolean;
  submitDisabled: boolean;
  offlineComputerName: string | null;
}

export interface SessionCreateDialogApi extends SessionCreateDialogState {
  onCancelEnvironment: () => void;
  onComputerChange: (next: RunOnTarget) => void;
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
  computerId: ComputerId,
  computers: ReturnType<typeof useComputers>["computers"]
): { isRemote: boolean; offlineComputerName: string | null; submitDisabled: boolean } {
  const isRemote = isRemoteComputerId(computerId);
  const selectedComputer = findComputerById(computers, computerId);
  return {
    isRemote,
    offlineComputerName:
      isRemote && selectedComputer?.status === "offline" ? selectedComputer.name : null,
    submitDisabled:
      isRemote && selectedComputer !== undefined && selectedComputer.status !== "connected",
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
  const userHomeDir = useUserHomeDir();
  const flow = useSelector(store, snapshot => snapshot.context);
  const { draft, pendingSubmit } = flow;
  const computerId = normalizeComputerId(draft.computerId);
  const computers = useComputers({ enabled: flow.open });
  const remoteState = resolveRemoteCreateAvailability(computerId, computers.computers);
  const pendingOp = pendingCreateOperation(flow.operation);
  // Each opening starts from the prefill; under All projects (or a remote computer) the picked
  // project (or User) is the destination, otherwise the project the opening named, else the
  // menubar destination.
  const target = useSessionCreateTarget({
    resetKey: flow.open,
    focusedBinding,
    seed: flow.target,
    computerId,
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
  const agentList = useSessionCreateAgents({
    computerId,
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
      flow.open && !remoteState.isRemote && destination.scope === "project" && binding !== null,
  });

  const submit = useSessionCreateSubmit({
    agents: agentList,
    binding,
    computerId,
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
    environment: destination.scope === "project" ? environment.field : undefined,
    environmentListingState: environment.listingState,
    environmentListingError: environment.listingError,
    isAwaitingEnvironment: pendingSubmit !== null,
    computerId,
    computers: computers.computerViews,
    localComputer: computers.localView,
    computerLocked: draft.sent || pendingOp.isSubmitting,
    submitDisabled: remoteState.submitDisabled,
    offlineComputerName: remoteState.offlineComputerName,
    onComputerChange: next => {
      if (next !== "auto") store.trigger.computerSelected({ computerId: next });
    },
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
  destination,
  target,
  open,
  agents,
}: {
  computerId: ComputerId;
  destination: ReturnType<typeof resolveSessionCreateDestination>;
  target: ReturnType<typeof useSessionCreateTarget>;
  open: boolean;
  agents: SessionCreateDialogContext["agents"];
}) {
  const userTarget = destination.scope === "user";
  const runtimeWorkspaceId = destination.runtimeWorkspaceId;
  const query = useAgents(userTarget ? null : (runtimeWorkspaceId ?? ""), {
    enabled: userTarget ? open : Boolean(runtimeWorkspaceId),
    profile: target.profile,
    computerId,
  });
  if (query.data) return query.data;
  // The shell's agents belong to its own project: never stand in for another target's.
  return !isRemoteComputerId(computerId) &&
    runtimeWorkspaceId &&
    !target.projectAxis &&
    !target.profileAxis &&
    !target.namedProject &&
    !target.namedProjectUnresolved
    ? (agents ?? [])
    : [];
}
