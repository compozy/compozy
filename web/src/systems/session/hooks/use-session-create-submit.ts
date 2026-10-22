import { use, useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSelector } from "@xstate/store-react";
import type { AgentPayload } from "@/systems/agent";
import { OsShellContext, type OsShellHandle } from "@/systems/os";
import type { TerminalQuote } from "@/systems/terminal/parts";
import { ownerDataScope, windowBindingValue, type DestinationKind } from "@/systems/workspace";

import { sessionCreateBinding } from "../lib/session-create-binding";
import { activateCreatedSessionWorkspace } from "../lib/session-create-navigation";
import { writeLastSessionProject } from "../lib/session-create-target";
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
import type { SessionCreateTarget } from "./use-session-create-target";

interface UseSessionCreateSubmitOptions {
  agents: readonly AgentPayload[];
  binding: ReturnType<typeof sessionCreateBinding>;
  environment: SessionEnvironmentModel;
  environmentWorkspaceId: string;
  runtimeWorkspaceId: string | null;
  scope: DestinationKind;
  store: SessionCreateStore;
  /** The inline picks: the profile that owns the session and the All-projects target. */
  target: Pick<SessionCreateTarget, "projectAxis" | "projectValue" | "profileAxis" | "profile">;
}

/** How a created session lands: in place under All projects, else today's switch. */
interface SessionLanding {
  scope: DestinationKind;
  currentWorkspaceId: string | null;
  /** The All-projects picker value to remember; `null` outside All projects. */
  allProjectsTarget: string | null;
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
  target,
}: UseSessionCreateSubmitOptions): () => void {
  const navigate = useNavigate();
  const shell = use(OsShellContext);
  const createSession = useCreateSession();
  const flow = useSelector(store, snapshot => snapshot.context);
  const pendingSubmit = flow.pendingSubmit;
  const worktreeId = environment.worktreeId;
  const createSessionAsync = createSession.mutateAsync;
  const profile = target.profileAxis ? target.profile : undefined;
  const allProjectsTarget = target.projectAxis ? target.projectValue : null;

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
          createSessionAsync({
            ...request,
            worktree: worktreeId,
            ...(profile ? { profile } : {}),
          }),
          pendingPrompt,
          terminalQuote
        ),
      navigate: session =>
        openCreatedSession(
          session,
          { scope, currentWorkspaceId: runtimeWorkspaceId, allProjectsTarget },
          navigate,
          shell
        ),
    });
  }, [
    allProjectsTarget,
    createSessionAsync,
    environment.status,
    navigate,
    pendingSubmit,
    profile,
    runtimeWorkspaceId,
    scope,
    shell,
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
    if (scope === "project") {
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
            ...(scope === "project" && worktreeId ? { worktree: worktreeId } : {}),
            ...(profile ? { profile } : {}),
          }),
          validated.pendingPrompt,
          terminalQuote
        ),
      navigate: session =>
        openCreatedSession(
          session,
          { scope, currentWorkspaceId: runtimeWorkspaceId, allProjectsTarget },
          navigate,
          shell
        ),
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
  const sessionName = flow.draft.sessionName.trim();
  const folderId = flow.draft.folderId.trim();
  return {
    ok: true,
    agentName,
    pendingPrompt: flow.pendingPrompt,
    request: {
      agent_name: agentName,
      ...binding,
      ...(sessionName ? { name: sessionName } : {}),
      ...(folderId ? { folder_id: folderId } : {}),
    },
  };
}

async function openCreatedSession(
  session: SessionPayload,
  landing: SessionLanding,
  navigate: ReturnType<typeof useNavigate>,
  shell: OsShellHandle | null
): Promise<void> {
  // Named before the workspace switch: the route reconciliation that lands it — after the
  // target workspace hydrates — prefers the operator's existing Session window (#825).
  shell?.coordinator.noteCreatedSession(session.id);
  if (landing.allProjectsTarget !== null) {
    // All projects stays on: the session opens in place, bound to its own scope.
    writeLastSessionProject(landing.allProjectsTarget);
    await navigate({
      to: "/agents/$name/sessions/$id",
      params: { name: session.agent_name, id: session.id },
      search: { workspace: windowBindingValue(ownerDataScope(session.workspace_id)) },
    });
    return;
  }
  activateCreatedSessionWorkspace(session, landing.currentWorkspaceId, {
    skip: landing.scope === "user",
  });
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
