import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSelector } from "@xstate/store-react";

import {
  networkParticipationDraftFromValues,
  networkParticipationValidationMessage,
  serializeNetworkParticipation,
} from "@/lib/network-participation";
import type { AgentPayload } from "@/systems/agent";
import type { TerminalQuote } from "@/systems/terminal/parts";
import type { WorkspaceScopeMode } from "@/systems/workspace";

import { sessionCreateBinding } from "../lib/session-create-binding";
import { activateCreatedSessionWorkspace } from "../lib/session-create-navigation";
import {
  claimPendingTerminalQuoteForCreate,
  restorePendingTerminalQuoteAfterFailedCreate,
  stageChosenSessionTerminalQuote,
} from "../lib/session-terminal-quote";
import { sessionStore } from "../stores/session-store";
import type { SessionCreateStore } from "../stores/session-create-store";
import type { CreateSessionParams, SessionPayload } from "../types";
import { useCreateSession } from "./use-session-actions";
import type { SessionEnvironmentModel } from "./use-session-environment";

interface UseSessionCreateSubmitOptions {
  agents: readonly AgentPayload[];
  binding: ReturnType<typeof sessionCreateBinding>;
  environment: SessionEnvironmentModel;
  environmentWorkspaceId: string;
  runtimeWorkspaceId: string | null;
  scope: WorkspaceScopeMode;
  store: SessionCreateStore;
}

type SessionCreateFlow = ReturnType<SessionCreateStore["getSnapshot"]>["context"];

type ValidatedSessionCreate =
  | { ok: false; message: string }
  | {
      ok: true;
      agentName: string;
      pendingPrompt: string | null;
      request: CreateSessionParams;
    };

export function useSessionCreateSubmit({
  agents,
  binding,
  environment,
  environmentWorkspaceId,
  runtimeWorkspaceId,
  scope,
  store,
}: UseSessionCreateSubmitOptions): () => void {
  const navigate = useNavigate();
  const createSession = useCreateSession();
  const flow = useSelector(store, snapshot => snapshot.context);
  const pendingSubmit = flow.pendingSubmit;
  const worktreeId = environment.worktreeId;
  const createSessionAsync = createSession.mutateAsync;

  useEffect(() => {
    if (!pendingSubmit) return;
    if (environment.status === "creating" || environment.status === "pending") return;
    if (environment.status !== "ready" || !worktreeId) {
      restorePendingTerminalQuoteAfterFailedCreate(pendingSubmit.terminalQuote);
      store.trigger.environmentSettled({});
      return;
    }
    const { agentName, pendingPrompt, request, terminalQuote, workspaceId } = pendingSubmit;
    store.trigger.submissionRequested({
      agentName,
      workspaceId,
      execute: () =>
        executeCreatedSession(
          createSessionAsync({ ...request, worktree: worktreeId }),
          pendingPrompt,
          terminalQuote
        ),
      navigate: session => openCreatedSession(session, runtimeWorkspaceId, scope, navigate),
    });
  }, [
    createSessionAsync,
    environment.status,
    navigate,
    pendingSubmit,
    runtimeWorkspaceId,
    scope,
    store,
    worktreeId,
  ]);

  return () => {
    if (flow.operation.status !== "idle" || pendingSubmit !== null) return;
    const validated = validateSessionCreate(flow, binding, agents);
    if (!validated.ok) {
      store.trigger.validationFailed({ message: validated.message });
      return;
    }
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
          agentName: validated.agentName,
          pendingPrompt: validated.pendingPrompt,
          previousEnvironment: (flow.draft.environment.kind === "new"
            ? flow.draft.environment.previous
            : undefined) ?? { kind: "root" },
          request: validated.request,
          terminalQuote: claimPendingTerminalQuoteForCreate(),
          workspaceId: runtimeWorkspaceId ?? environmentWorkspaceId,
        });
        return;
      }
    }
    const terminalQuote = claimPendingTerminalQuoteForCreate();
    store.trigger.submissionRequested({
      agentName: validated.agentName,
      workspaceId: runtimeWorkspaceId ?? "",
      execute: () =>
        executeCreatedSession(
          createSessionAsync({
            ...validated.request,
            ...(scope === "workspace" && worktreeId ? { worktree: worktreeId } : {}),
          }),
          validated.pendingPrompt,
          terminalQuote
        ),
      navigate: session => openCreatedSession(session, runtimeWorkspaceId, scope, navigate),
    });
  };
}

function validateSessionCreate(
  flow: SessionCreateFlow,
  binding: ReturnType<typeof sessionCreateBinding>,
  agents: readonly AgentPayload[]
): ValidatedSessionCreate {
  if (!binding) return { ok: false, message: "Choose a destination before starting the session." };
  const agentName = flow.draft.agentName.trim();
  if (agentName.length === 0 || !agents.some(agent => agent.name === agentName)) {
    return { ok: false, message: "Select an agent before starting the session." };
  }
  const participation = networkParticipationDraftFromValues(
    flow.draft.networkParticipationMode,
    flow.draft.networkChannelId,
    flow.draft.networkChannelStrategy
  );
  const participationError = networkParticipationValidationMessage(participation, ["named"]);
  if (participationError) return { ok: false, message: participationError };
  const sessionName = flow.draft.sessionName.trim();
  return {
    ok: true,
    agentName,
    pendingPrompt: flow.pendingPrompt,
    request: {
      agent_name: agentName,
      ...binding,
      ...(sessionName ? { name: sessionName } : {}),
      network_participation: serializeNetworkParticipation(participation),
    },
  };
}

async function openCreatedSession(
  session: SessionPayload,
  currentWorkspaceId: string | null,
  scope: WorkspaceScopeMode,
  navigate: ReturnType<typeof useNavigate>
): Promise<void> {
  activateCreatedSessionWorkspace(session, currentWorkspaceId, { skip: scope === "global" });
  await navigate({
    to: "/agents/$name/sessions/$id",
    params: { name: session.agent_name, id: session.id },
  });
}

async function executeCreatedSession(
  create: Promise<SessionPayload>,
  pendingPrompt: string | null,
  quote: TerminalQuote | null
): Promise<SessionPayload> {
  try {
    const session = await create;
    if (quote) stageChosenSessionTerminalQuote(session.id, quote);
    if (pendingPrompt?.trim()) {
      sessionStore.trigger.firstPromptQueued({ sessionId: session.id, text: pendingPrompt });
    }
    return session;
  } catch (error) {
    restorePendingTerminalQuoteAfterFailedCreate(quote);
    throw error;
  }
}
