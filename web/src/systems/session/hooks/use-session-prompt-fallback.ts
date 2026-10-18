import { notifyUser } from "@/lib/user-feedback";
import { useAgents } from "@/systems/agent";
import { useActiveWorkspace } from "@/systems/workspace";

import { clearPendingTerminalQuote } from "../lib/session-terminal-quote";
import { sessionStore } from "../stores/session-store";
import type { SessionPayload } from "../types";
import { useCreateSession } from "./use-session-actions";
import { useSessionCreateIsCreating, useSessionCreateStore } from "./use-session-create";

export interface SessionPromptFallbackOptions {
  onCreated(session: SessionPayload & { workspace_id: string }): void;
  onPickerOpened(): void;
}

export interface SessionPromptFallbackRunOptions {
  /** Starts with this agent instead of the workspace default; an unknown name opens the picker. */
  agentName?: string;
  /** Binds the session to this ready worktree instead of the workspace root. */
  worktreeId?: string;
}

/**
 * What one run did with the query: a session now owns it, the create dialog
 * holds it, creation failed (already reported to the operator), or the run was
 * refused before anything started (blank query or another creation in flight).
 */
export type SessionPromptFallbackOutcome = "created" | "picker" | "failed" | "ignored";

export interface SessionPromptFallback {
  pending: boolean;
  run(
    query: string,
    options?: SessionPromptFallbackRunOptions
  ): Promise<SessionPromptFallbackOutcome>;
}

/**
 * Turns one explicit prompt submission into a session. Merely rendering or typing never calls a
 * prompt transport; the first-message queue is armed only after the session has a durable id.
 *
 * In-flight and pending come from the session-create store — the same authority the dialog uses —
 * so a fallback never invents a second lifecycle. Success returns to idle without dialog
 * navigation.
 */
export function useSessionPromptFallback({
  onCreated,
  onPickerOpened,
}: SessionPromptFallbackOptions): SessionPromptFallback {
  const workspace = useActiveWorkspace();
  const createDialog = useSessionCreateStore();
  const createSession = useCreateSession();
  const pending = useSessionCreateIsCreating();
  const agents = useAgents(workspace.runtimeWorkspaceId ?? "", {
    enabled: workspace.runtimeWorkspaceId !== null,
  });

  return {
    pending,
    run: async (query, options = {}) => {
      if (query.trim() === "" || createDialog.getSnapshot().context.operation.status !== "idle") {
        return "ignored";
      }
      clearPendingTerminalQuote();
      const workspaceId = workspace.runtimeWorkspaceId;
      if (workspaceId === null) {
        notifyUser({ message: "The active workspace is not ready.", tone: "error" });
        return "failed";
      }

      const agentName =
        options.agentName?.trim() || workspace.runtimeWorkspace?.default_agent?.trim() || "";
      const worktreeId = options.worktreeId?.trim() || undefined;
      const agentResolves =
        agentName !== "" &&
        agents.isSuccess &&
        (agents.data?.some(agent => agent.name === agentName) ?? false);
      if (!agentResolves) {
        createDialog.trigger.dialogOpened({
          agentName: "",
          workspaceId,
          ...(worktreeId ? { environment: { kind: "worktree" as const, worktreeId } } : {}),
        });
        createDialog.trigger.fallbackPromptStaged({ prompt: query });
        onPickerOpened();
        return "picker";
      }

      createDialog.trigger.fallbackRequested({ agentName, workspaceId });
      const operation = createDialog.getSnapshot().context.operation;
      if (operation.status !== "submitting") return "ignored";
      const attempt = operation.attempt;
      try {
        const session = await createSession.mutateAsync({
          agent_name: agentName,
          workspace: workspaceId,
          ...(worktreeId ? { worktree: worktreeId } : {}),
        });
        sessionStore.trigger.firstPromptQueued({ sessionId: session.id, text: query });
        onCreated({ ...session, workspace_id: session.workspace_id ?? workspaceId });
        createDialog.trigger.fallbackCompleted({ attempt });
        return "created";
      } catch (error) {
        const reason = error instanceof Error ? error.message : "The session could not be created.";
        createDialog.trigger.submissionFailed({
          attempt,
          message: `Could not ask the agent: ${reason}`,
        });
        return "failed";
      }
    },
  };
}
