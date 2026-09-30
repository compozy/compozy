import { useEffect, useState } from "react";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { notifyUser } from "@/lib/user-feedback";
import { useAgents, type AgentPayload } from "@/systems/agent";
import { useSessionCreateStore, useSessionPromptFallback } from "@/systems/session";
import { useActiveWorkspace, useScopedWorktreeFilter } from "@/systems/workspace";

import { useAttentionJump } from "./use-attention-jump";

/** Clears a sent prompt from the composer once a session owns it. */
export type ReleasePrompt = () => void;

export interface EmptyDesktopSession {
  /** The project a prompt starts in; `null` in Global scope. */
  workspaceId: string | null;
  /** A session is being created — from here or any other launcher sharing the create flow. */
  pending: boolean;
  agents: AgentPayload[];
  agentsLoading: boolean;
  agentsError: string | null;
  /** The agent the next prompt starts with: the operator's pick, else the project default. */
  agentName: string | null;
  selectAgent(agentName: string | null): void;
  /**
   * Starts a session with `prompt` as its first message and opens it on this
   * desktop, calling `release` once a session owns the prompt. A failed start —
   * or a create dialog dismissed after the prompt moved there — never releases
   * it, so the words stay in the composer.
   */
  start(prompt: string, release: ReleasePrompt): Promise<void>;
}

/**
 * The empty desktop's launcher: the palette's prompt-first create path
 * (`useSessionPromptFallback`) pointed at the operator's agent and this
 * window scope's worktree, landing the new session through the same jump
 * every attention surface uses.
 */
export function useEmptyDesktopSession(): EmptyDesktopSession {
  const { runtimeWorkspace, runtimeWorkspaceId, scope } = useActiveWorkspace();
  const scopeId = useWorktreeScopeId();
  const scopedWorktree = useScopedWorktreeFilter(
    scope === "workspace" ? runtimeWorkspaceId : null,
    scopeId
  );
  const agents = useAgents(runtimeWorkspaceId ?? "", { enabled: runtimeWorkspaceId !== null });
  const createStore = useSessionCreateStore();
  const jumpToSession = useAttentionJump();
  // A pick belongs to the project it was made in; another project starts from its own default.
  const [choice, setChoice] = useState<{ workspaceId: string | null; agentName: string } | null>(
    null
  );
  // A prompt handed to the create dialog (no agent resolved) is released only
  // if the dialog goes on to create the session.
  const [heldPrompt, setHeldPrompt] = useState<{ release: ReleasePrompt } | null>(null);
  const fallback = useSessionPromptFallback({
    onCreated: session =>
      jumpToSession({
        sessionId: session.id,
        agentName: session.agent_name,
        workspaceId: session.workspace_id,
      }),
    onPickerOpened: () => undefined,
  });

  useEffect(() => {
    if (heldPrompt === null) return;
    const subscription = createStore.subscribe(snapshot => {
      if (snapshot.context.open) return;
      // Closed while idle: dismissed, and the prompt stays. Anything else is a creation under way.
      subscription.unsubscribe();
      if (snapshot.context.operation.status !== "idle") heldPrompt.release();
      setHeldPrompt(null);
    });
    return () => subscription.unsubscribe();
  }, [createStore, heldPrompt]);

  const catalog = agents.data ?? [];
  const chosen =
    choice !== null && choice.workspaceId === runtimeWorkspaceId ? choice.agentName : null;
  const defaultAgent = runtimeWorkspace?.default_agent?.trim() || null;

  return {
    workspaceId: runtimeWorkspaceId,
    pending: fallback.pending,
    agents: catalog,
    agentsLoading: agents.isLoading,
    agentsError: agents.error instanceof Error ? agents.error.message : null,
    agentName: chosen ?? defaultAgent,
    selectAgent: agentName =>
      setChoice(agentName ? { workspaceId: runtimeWorkspaceId, agentName } : null),
    start: async (prompt, release) => {
      if (!scopedWorktree.resolved) {
        notifyUser({ message: "Worktrees are not available yet. Try again.", tone: "error" });
        return;
      }
      const outcome = await fallback.run(prompt, {
        agentName: chosen ?? undefined,
        worktreeId: scopedWorktree.worktreeId,
      });
      if (outcome === "created") release();
      if (outcome === "picker") setHeldPrompt({ release });
    },
  };
}
