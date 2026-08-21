import { notifyUser } from "@/lib/user-feedback";
import { useAgents } from "@/systems/agent";
import { useActiveWorkspace } from "@/systems/workspace";

import { sessionStore } from "../stores/session-store";
import type { SessionPayload } from "../types";
import { useCreateSession } from "./use-session-actions";
import { useSessionCreateIsCreating, useSessionCreateStore } from "./use-session-create";

export interface SessionPromptFallbackOptions {
  onCreated(session: SessionPayload & { workspace_id: string }): void;
  onPickerOpened(): void;
}

export interface SessionPromptFallback {
  pending: boolean;
  run(query: string): Promise<void>;
}

/**
 * Turns one explicit palette selection into a session. Merely rendering or typing never calls a
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
    run: async query => {
      if (query.trim() === "" || createDialog.getSnapshot().context.operation.status !== "idle") {
        return;
      }
      const workspaceId = workspace.runtimeWorkspaceId;
      if (workspaceId === null) {
        notifyUser({ message: "The active workspace is not ready.", tone: "error" });
        return;
      }

      const defaultAgent = workspace.runtimeWorkspace?.default_agent?.trim() ?? "";
      const defaultResolves =
        defaultAgent !== "" &&
        agents.isSuccess &&
        (agents.data?.some(agent => agent.name === defaultAgent) ?? false);
      if (!defaultResolves) {
        createDialog.trigger.dialogOpened({ agentName: "", workspaceId });
        createDialog.trigger.firstMessageChanged({ firstMessage: query });
        onPickerOpened();
        return;
      }

      createDialog.trigger.fallbackRequested({ agentName: defaultAgent, workspaceId });
      const operation = createDialog.getSnapshot().context.operation;
      if (operation.status !== "submitting") return;
      const attempt = operation.attempt;
      try {
        const session = await createSession.mutateAsync({
          agent_name: defaultAgent,
          workspace: workspaceId,
        });
        sessionStore.trigger.firstPromptQueued({ sessionId: session.id, text: query });
        onCreated({ ...session, workspace_id: session.workspace_id ?? workspaceId });
        createDialog.trigger.fallbackCompleted({ attempt });
      } catch (error) {
        const reason = error instanceof Error ? error.message : "The session could not be created.";
        createDialog.trigger.submissionFailed({
          attempt,
          message: `Could not ask the agent: ${reason}`,
        });
      }
    },
  };
}
