import { useSelector, useStore } from "@xstate/store-react";

import type { EntityMode } from "@compozy/ui";

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
}

export interface SessionCreateDialogApi extends SessionCreateDialogState {
  onCancelEnvironment: () => void;
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
  const { draft } = flow;
  const { isSubmitting, pendingAgentName, pendingWorkspaceId } = pendingCreateOperation(
    flow.operation
  );
  const { pendingSubmit } = flow;
  // Each opening starts from the prefill; under All projects the picked project
  // (or User) is the destination, otherwise the menubar destination is.
  const target = useSessionCreateTarget({ resetKey: flow.open, focusedBinding });
  const destination = resolveSessionCreateDestination(
    target.projectAxis
      ? {
          activeWorkspace: target.workspace,
          projectWorkspaceId: target.workspace?.id ?? null,
          dataScope: target.scope,
          userHomeDir,
        }
      : { activeWorkspace, projectWorkspaceId, dataScope, userHomeDir }
  );
  const agentList = useSessionCreateAgents({ destination, target, open: flow.open, agents });
  const selectedAgentName = draft.agentName;
  const environment = useSessionEnvironment({
    workspaceId: destination.environmentWorkspaceId,
    target: draft.environment,
    onTargetChange: next => {
      restorePendingTerminalQuoteAfterFailedCreate(
        store.getSnapshot().context.pendingSubmit?.terminalQuote ?? null
      );
      store.trigger.environmentSelected({ environment: next });
    },
    enabled: flow.open && destination.scope === "project" && destination.binding !== null,
  });

  const submit = useSessionCreateSubmit({
    agents: agentList,
    binding: destination.binding,
    environment,
    environmentWorkspaceId: destination.environmentWorkspaceId,
    runtimeWorkspaceId: destination.runtimeWorkspaceId,
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
    destinationReady: destination.binding !== null,
    sessionName: draft.sessionName,
    selectedAgentName,
    isSubmitting,
    submitError: flow.submitError,
    pendingAgentName,
    pendingWorkspaceId,
    userHomeDir,
    environment: destination.scope === "project" ? environment.field : undefined,
    environmentListingState: environment.listingState,
    environmentListingError: environment.listingError,
    isAwaitingEnvironment: pendingSubmit !== null,
    destinationAxes: {
      ...(target.projectAxis && target.projectValue !== null
        ? {
            project: {
              options: target.projectOptions,
              value: target.projectValue,
              onChange: (value: string) => {
                if (value === target.projectValue) return;
                target.selectProject(value);
                // Worktrees belong to one project: a new project starts at its root.
                store.trigger.environmentSelected({ environment: ROOT_ENVIRONMENT_TARGET });
              },
            },
          }
        : {}),
      ...(target.profileAxis
        ? {
            profile: {
              options: target.profileOptions,
              value: target.profile,
              onChange: target.selectProfile,
            },
          }
        : {}),
    },
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
  destination,
  target,
  open,
  agents,
}: {
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
  });
  if (query.data) return query.data;
  return runtimeWorkspaceId && !target.projectAxis && !target.profileAxis ? (agents ?? []) : [];
}
