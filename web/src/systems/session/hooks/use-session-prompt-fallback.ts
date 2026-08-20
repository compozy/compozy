import { useRef, useState } from "react";

import { notifyUser } from "@/lib/user-feedback";
import { useAgents } from "@/systems/agent";
import { useActiveWorkspace } from "@/systems/workspace";

import { sessionStore } from "../stores/session-store";
import type { SessionPayload } from "../types";
import { useCreateSession } from "./use-session-actions";
import { useSessionCreateStore } from "./use-session-create";

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
 */
export function useSessionPromptFallback({
  onCreated,
  onPickerOpened,
}: SessionPromptFallbackOptions): SessionPromptFallback {
  const workspace = useActiveWorkspace();
  const createDialog = useSessionCreateStore();
  const createSession = useCreateSession();
  const agents = useAgents(workspace.runtimeWorkspaceId ?? "", {
    enabled: workspace.runtimeWorkspaceId !== null,
  });
  const inFlight = useRef(false);
  const [pending, setPending] = useState(false);

  return {
    pending,
    run: async query => {
      if (query.trim() === "" || inFlight.current) return;
      const workspaceId = workspace.runtimeWorkspaceId;
      if (workspaceId === null) {
        notifyUser({ message: "The active workspace is not ready.", tone: "error" });
        return;
      }

      const defaultAgent = workspace.runtimeWorkspace?.default_agent?.trim() ?? "";
      const defaultResolves =
        defaultAgent !== "" &&
        (agents.data === undefined || agents.data.some(agent => agent.name === defaultAgent));
      if (!defaultResolves) {
        createDialog.trigger.dialogOpened({ agentName: "", workspaceId });
        createDialog.trigger.firstMessageChanged({ firstMessage: query });
        onPickerOpened();
        return;
      }

      inFlight.current = true;
      setPending(true);
      try {
        const session = await createSession.mutateAsync({
          agent_name: defaultAgent,
          workspace: workspaceId,
        });
        sessionStore.trigger.firstPromptQueued({ sessionId: session.id, text: query });
        onCreated({ ...session, workspace_id: session.workspace_id ?? workspaceId });
      } catch (error) {
        const reason = error instanceof Error ? error.message : "The session could not be created.";
        notifyUser({
          message: `Could not ask the agent: ${reason}`,
          tone: "error",
        });
      } finally {
        inFlight.current = false;
        setPending(false);
      }
    },
  };
}
