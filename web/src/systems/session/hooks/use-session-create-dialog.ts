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
import { useCreateSession } from "./use-session-actions";
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
}

export interface SessionCreateDialogApi extends SessionCreateDialogState {
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

export function useSessionCreateDialogController(): SessionCreateDialogController {
  return { store: useStore(sessionCreateStoreLogic) };
}

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

  const workspaceAgentsQuery = useAgents(runtimeWorkspaceId ?? "", {
    enabled: runtimeWorkspaceId !== null && runtimeWorkspaceId.length > 0,
  });
  const agentList = workspaceAgentsQuery.data ?? (runtimeWorkspaceId ? (agents ?? []) : []);
  const selectedAgentName = draft.agentName;

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

  const submit = () => {
    if (!binding || isSubmitting) return;
    const agentName = selectedAgentName.trim();
    if (agentName.length === 0) {
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
    store.trigger.submissionRequested({
      agentName,
      workspaceId: runtimeWorkspaceId ?? "",
      execute: () =>
        createSession.mutateAsync({
          agent_name: agentName,
          ...binding,
          ...(sessionName.length > 0 ? { name: sessionName } : {}),
          network_participation: serializeNetworkParticipation(networkParticipation),
        }),
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
    onOpenChange: open => store.trigger.dialogOpenChanged({ open }),
    onModeChange: mode => store.trigger.modeSelected({ mode }),
    onAgentChange: agentName =>
      store.trigger.agentSelected({ agentName, workspaceId: runtimeWorkspaceId ?? "" }),
    onSessionNameChange: sessionName => store.trigger.sessionNameChanged({ sessionName }),
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
