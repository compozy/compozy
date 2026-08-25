import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSelector, useStore } from "@xstate/store-react";

import type { EntityMode } from "@compozy/ui";

import {
  networkParticipationDraftFromValues,
  networkParticipationValidationMessage,
  serializeNetworkParticipation,
  type NetworkParticipationDraft,
} from "@/lib/network-participation";

import type { SessionCreateDialogDraft } from "../lib/session-create-draft";
import { sessionCreateBinding } from "../lib/session-create-binding";
import { activateCreatedSessionWorkspace } from "../lib/session-create-navigation";
import { sessionCreateStoreLogic, type SessionCreateStore } from "../stores/session-create-store";
import { sessionStore } from "../stores/session-store";
import type { CreateSessionParams } from "../types";
import { useCreateSession } from "./use-session-actions";
import { useSessionEnvironment, type SessionEnvironmentModel } from "./use-session-environment";
import { type AgentPayload, useAgents } from "@/systems/agent";
import {
  GLOBAL_SCOPE_COPY,
  destinationLabel,
  useUserHomeDir,
  type WorkspaceScopeMode,
  type WorkspacePayload,
} from "@/systems/workspace";

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
  firstMessage: string;
  /** True while a submit the operator already asked for waits on its worktree. */
  isAwaitingEnvironment: boolean;
}

export interface SessionCreateDialogApi extends SessionCreateDialogState {
  onCancelEnvironment: () => void;
  onOpenChange: (open: boolean) => void;
  onModeChange: (mode: EntityMode) => void;
  onAgentChange: (agentName: string) => void;
  onSessionNameChange: (next: string) => void;
  onFirstMessageChange: (next: string) => void;
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
  const navigate = useNavigate();
  const createSession = useCreateSession();
  const userHomeDir = useUserHomeDir();
  const flow = useSelector(store, snapshot => snapshot.context);
  const { draft } = flow;
  const isSubmitting = flow.operation.status === "submitting";
  const navigationTarget =
    flow.operation.status === "navigation-pending" ? flow.operation.target : null;
  const pendingAgentName = flow.operation.status === "submitting" ? flow.operation.agentName : null;
  const pendingWorkspaceId =
    flow.operation.status === "submitting" ? flow.operation.workspaceId : null;
  const { pendingSubmit } = flow;
  const scope: WorkspaceScopeMode = requestedScope ?? (activeWorkspace ? "workspace" : "global");
  const runtimeWorkspaceId = activeWorkspace?.id ?? null;
  const binding = sessionCreateBinding({
    scope,
    projectWorkspaceId:
      scope === "workspace"
        ? (projectWorkspaceId ?? runtimeWorkspaceId)
        : (projectWorkspaceId ?? null),
    homeWorkspaceId:
      homeWorkspaceId ?? (scope === "global" ? (runtimeWorkspaceId ?? undefined) : undefined),
    userHomeDir,
  });
  const destinationReady = binding !== null;
  const sessionRoot = scope === "global" ? (userHomeDir ?? "~") : (activeWorkspace?.root_dir ?? "");
  const landingLabel = destinationLabel(
    scope,
    scope === "global" ? GLOBAL_SCOPE_COPY.chipLabel : activeWorkspace?.name
  );
  const environmentWorkspaceId =
    scope === "workspace" ? (projectWorkspaceId ?? runtimeWorkspaceId ?? "") : "";

  const workspaceAgentsQuery = useAgents(runtimeWorkspaceId ?? "", {
    enabled: runtimeWorkspaceId !== null && runtimeWorkspaceId.length > 0,
  });
  const agentList = workspaceAgentsQuery.data ?? (runtimeWorkspaceId ? (agents ?? []) : []);
  const selectedAgentName = draft.agentName;
  const environment = useSessionEnvironment({
    workspaceId: environmentWorkspaceId,
    target: draft.environment,
    onTargetChange: next => store.trigger.environmentSelected({ environment: next }),
    enabled: flow.open && scope === "workspace" && destinationReady,
  });

  useEffect(() => {
    if (!navigationTarget) return;
    const { attempt, session } = navigationTarget;
    store.trigger.navigationRequested({
      attempt,
      execute: async () => {
        activateCreatedSessionWorkspace(session, runtimeWorkspaceId, {
          skip: scope === "global",
        });
        await navigate({
          to: "/agents/$name/sessions/$id",
          params: { name: session.agent_name, id: session.id },
        });
      },
    });
  }, [navigate, navigationTarget, runtimeWorkspaceId, scope, store]);

  // `mutateAsync` is stable across renders while the mutation object is not, so
  // the armed-submit effect can depend on it without re-entering every keystroke.
  const createSessionAsync = createSession.mutateAsync;
  const worktreeId = environment.worktreeId;

  const previousEnvironment =
    draft.environment.kind === "new" ? draft.environment.previous : undefined;

  // The worktree is created by this submit, so the request outlives the click.
  // Everything needed to finish it was captured when it was armed, which is why
  // this can resume without reading the draft the operator may still be editing.
  useEffect(() => {
    if (!pendingSubmit) return;
    if (environment.status === "creating" || environment.status === "pending") return;
    if (environment.status !== "ready" || !worktreeId) {
      store.trigger.environmentSettled({});
      return;
    }
    const { agentName, firstMessage, request, workspaceId: submitWorkspaceId } = pendingSubmit;
    store.trigger.submissionRequested({
      agentName,
      workspaceId: submitWorkspaceId,
      execute: async () => {
        const session = await createSessionAsync({ ...request, worktree: worktreeId });
        sessionStore.trigger.firstPromptQueued({ sessionId: session.id, text: firstMessage });
        return session;
      },
    });
  }, [createSessionAsync, environment.status, pendingSubmit, store, worktreeId]);

  const submit = () => {
    if (!binding || isSubmitting || pendingSubmit !== null) return;
    const agentName = selectedAgentName.trim();
    if (agentName.length === 0) {
      store.trigger.validationFailed({ message: "Select an agent before starting the session." });
      return;
    }
    if (!agentList.some(agent => agent.name === agentName)) {
      store.trigger.validationFailed({ message: "Select an agent before starting the session." });
      return;
    }

    const networkParticipation = networkParticipationDraftFromValues(
      draft.networkParticipationMode,
      draft.networkChannelId,
      draft.networkChannelStrategy
    );
    const participationError = networkParticipationValidationMessage(networkParticipation, [
      "named",
    ]);
    if (participationError) {
      store.trigger.validationFailed({ message: participationError });
      return;
    }

    const sessionName = draft.sessionName.trim();
    const request: CreateSessionParams = {
      agent_name: agentName,
      ...binding,
      ...(sessionName.length > 0 ? { name: sessionName } : {}),
      network_participation: serializeNetworkParticipation(networkParticipation),
    };
    const firstMessage = draft.firstMessage;

    if (scope === "workspace") {
      const readiness = environment.ensureReady();
      if (readiness === "blocked") {
        store.trigger.validationFailed({
          message: "Choose an available environment before starting the session.",
        });
        return;
      }
      if (readiness === "materializing") {
        store.trigger.environmentAwaited({
          agentName,
          firstMessage,
          previousEnvironment: previousEnvironment ?? { kind: "root" },
          request,
          workspaceId: runtimeWorkspaceId ?? environmentWorkspaceId,
        });
        return;
      }
    }

    store.trigger.submissionRequested({
      agentName,
      workspaceId: runtimeWorkspaceId ?? "",
      execute: async () => {
        const session = await createSessionAsync({
          ...request,
          ...(scope === "workspace" && worktreeId ? { worktree: worktreeId } : {}),
        });
        // Queued against the durable id, inside the one call that produced it,
        // so the message can neither precede the session nor be sent twice.
        sessionStore.trigger.firstPromptQueued({ sessionId: session.id, text: firstMessage });
        return session;
      },
    });
  };

  return {
    open: flow.open,
    restoreFocusOnClose: flow.restoreFocusOnClose,
    mode: flow.mode,
    agents: agentList,
    workspace: activeWorkspace,
    workspaceId: runtimeWorkspaceId,
    scope,
    destinationLabel: landingLabel,
    sessionRoot,
    destinationReady,
    sessionName: draft.sessionName,
    selectedAgentName,
    isSubmitting,
    submitError: flow.submitError,
    pendingAgentName,
    pendingWorkspaceId,
    userHomeDir,
    environment: scope === "workspace" ? environment.field : undefined,
    environmentListingState: environment.listingState,
    environmentListingError: environment.listingError,
    firstMessage: draft.firstMessage,
    isAwaitingEnvironment: pendingSubmit !== null,
    onCancelEnvironment: () => {
      // Abandons the checkout without dismissing: what was typed is still the
      // operator's, and they only asked to stop creating a worktree.
      const restoreTarget = pendingSubmit?.previousEnvironment ?? { kind: "root" };
      environment.cancelPending(restoreTarget);
      store.trigger.environmentRestored({ environment: restoreTarget });
    },
    onOpenChange: open => {
      // Dismissing mid-creation must not strand a half-built worktree.
      if (!open) environment.cancelPending();
      store.trigger.dialogOpenChanged({ open });
    },
    onModeChange: mode => store.trigger.modeSelected({ mode }),
    onAgentChange: agentName =>
      store.trigger.agentSelected({ agentName, workspaceId: runtimeWorkspaceId ?? "" }),
    onSessionNameChange: sessionName => store.trigger.sessionNameChanged({ sessionName }),
    onFirstMessageChange: firstMessage => store.trigger.firstMessageChanged({ firstMessage }),
    onNetworkParticipationChange: next =>
      store.trigger.networkParticipationSelected({
        networkParticipationMode: next.mode,
        networkChannelId: next.channelId,
        networkChannelStrategy: next.channelStrategy,
      }),
    networkParticipation: networkParticipationDraftFromValues(
      draft.networkParticipationMode,
      draft.networkChannelId,
      draft.networkChannelStrategy
    ),
    submit,
  };
}

export type { SessionCreateDialogDraft };
