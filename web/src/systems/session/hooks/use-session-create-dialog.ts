import { useSelector, useStore } from "@xstate/store-react";

import type { EntityMode } from "@compozy/ui";

import {
  networkParticipationDraftFromValues,
  type NetworkParticipationDraft,
} from "@/lib/network-participation";

import type { SessionCreateDialogDraft } from "../lib/session-create-draft";
import { restorePendingTerminalQuoteAfterFailedCreate } from "../lib/session-terminal-quote";
import { resolveSessionCreateDestination } from "../lib/session-create-destination";
import { sessionCreateStoreLogic, type SessionCreateStore } from "../stores/session-create-store";
import { useSessionEnvironment, type SessionEnvironmentModel } from "./use-session-environment";
import { type AgentPayload, useAgents } from "@/systems/agent";
import {
  useUserHomeDir,
  type WorkspaceScopeMode,
  type WorkspacePayload,
} from "@/systems/workspace";
import { useSessionCreateSubmit } from "./use-session-create-submit";
import { useSessionCreateDialogActions } from "./use-session-create-dialog-actions";

interface SessionCreateDialogContext {
  agents: AgentPayload[] | undefined;
  activeWorkspace: WorkspacePayload | undefined;
  scope?: WorkspaceScopeMode;
  projectWorkspaceId?: string | null;
  homeWorkspaceId?: string;
}

export interface SessionCreateDialogState {
  open: boolean;
  restoreFocusOnClose: boolean;
  mode: EntityMode;
  agents: AgentPayload[];
  workspace: WorkspacePayload | undefined;
  workspaceId: string | null;
  scope: WorkspaceScopeMode;
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
}

export interface SessionCreateDialogApi extends SessionCreateDialogState {
  onCancelEnvironment: () => void;
  onOpenChange: (open: boolean) => void;
  onModeChange: (mode: EntityMode) => void;
  onAgentChange: (agentName: string) => void;
  onSessionNameChange: (next: string) => void;
  onNetworkParticipationChange: (next: NetworkParticipationDraft) => void;
  networkParticipation: NetworkParticipationDraft;
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
    scope: requestedScope,
    projectWorkspaceId,
    homeWorkspaceId,
  }: SessionCreateDialogContext,
  store: SessionCreateStore
): SessionCreateDialogApi {
  const userHomeDir = useUserHomeDir();
  const flow = useSelector(store, snapshot => snapshot.context);
  const { draft } = flow;
  const isSubmitting = flow.operation.status === "submitting";
  const pendingAgentName = flow.operation.status === "submitting" ? flow.operation.agentName : null;
  const pendingWorkspaceId =
    flow.operation.status === "submitting" ? flow.operation.workspaceId : null;
  const { pendingSubmit } = flow;
  const destination = resolveSessionCreateDestination({
    activeWorkspace,
    homeWorkspaceId,
    projectWorkspaceId,
    requestedScope,
    userHomeDir,
  });

  const workspaceAgentsQuery = useAgents(destination.runtimeWorkspaceId ?? "", {
    enabled: destination.runtimeWorkspaceId !== null && destination.runtimeWorkspaceId.length > 0,
  });
  const agentList =
    workspaceAgentsQuery.data ?? (destination.runtimeWorkspaceId ? (agents ?? []) : []);
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
    enabled: flow.open && destination.scope === "workspace" && destination.binding !== null,
  });

  const submit = useSessionCreateSubmit({
    agents: agentList,
    binding: destination.binding,
    environment,
    environmentWorkspaceId: destination.environmentWorkspaceId,
    runtimeWorkspaceId: destination.runtimeWorkspaceId,
    scope: destination.scope,
    store,
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
    environment: destination.scope === "workspace" ? environment.field : undefined,
    environmentListingState: environment.listingState,
    environmentListingError: environment.listingError,
    isAwaitingEnvironment: pendingSubmit !== null,
    ...actions,
    networkParticipation: networkParticipationDraftFromValues(
      draft.networkParticipationMode,
      draft.networkChannelId,
      draft.networkChannelStrategy
    ),
    submit,
  };
}

export type { SessionCreateDialogDraft };
