import type { EmptyDesktopPromptRuntimeOptions } from "./use-empty-desktop-prompt-runtime";
import { useEffect, useRef, useState } from "react";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { notifyUser } from "@/lib/user-feedback";
import { useAgents, type AgentPayload } from "@/systems/agent";
import {
  useSessionCreateActions,
  useSessionCreateStore,
  useSessionPromptFallback,
} from "@/systems/session";
import { useActiveWorkspace, useScopedWorktreeFilter } from "@/systems/workspace";

import { useAttentionJump } from "./use-attention-jump";
import { useOsShell } from "./use-os-shell";

/** Clears a sent prompt from the composer once a session owns it. */
export type ReleasePrompt = () => void;

export interface EmptyDesktopSession {
  /** The project a prompt starts in; `null` outside a project. */
  workspaceId: string | null;
  /**
   * No project is focused but All projects is on: a sent prompt opens New
   * session, which asks for the project (and profile) inline (ADR-003).
   */
  asksTarget: boolean;
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
  start: EmptyDesktopPromptRuntimeOptions["onSubmit"];
}

/**
 * The empty desktop's launcher: the palette's prompt-first create path
 * (`useSessionPromptFallback`) pointed at the operator's agent and this
 * window scope's worktree, landing the new session through the same jump
 * every attention surface uses.
 */
export function useEmptyDesktopSession(): EmptyDesktopSession {
  const { runtimeWorkspace, runtimeWorkspaceId } = useActiveWorkspace();
  const scopeId = useWorktreeScopeId();
  const scopedWorktree = useScopedWorktreeFilter(runtimeWorkspaceId, scopeId);
  const agents = useAgents(runtimeWorkspaceId ?? "", { enabled: runtimeWorkspaceId !== null });
  const createStore = useSessionCreateStore();
  const { canStart, openWithPrompt } = useSessionCreateActions();
  const asksTarget = runtimeWorkspaceId === null && canStart;
  const jumpToSession = useAttentionJump();
  const { manager } = useOsShell();
  // The desktop the prompt was sent from: the session lands there even if the
  // operator switches desktops while it is being created.
  const originDesktopRef = useRef<string | null>(null);
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
        desktopId: originDesktopRef.current ?? undefined,
        created: true,
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
    asksTarget,
    pending: fallback.pending,
    agents: catalog,
    agentsLoading: agents.isLoading,
    agentsError: agents.error instanceof Error ? agents.error.message : null,
    agentName: chosen ?? defaultAgent,
    selectAgent: agentName =>
      setChoice(agentName ? { workspaceId: runtimeWorkspaceId, agentName } : null),
    start: async (prompt, release, options) => {
      if (asksTarget) {
        // The prompt waits in New session until the operator picks where it runs.
        openWithPrompt(prompt);
        setHeldPrompt({ release });
        return;
      }
      if (!scopedWorktree.resolved) {
        notifyUser({ message: "Worktrees are not available yet. Try again.", tone: "error" });
        return;
      }
      originDesktopRef.current = manager.getState().activeDesktopId;
      const outcome = await fallback.run(prompt, {
        agentName: chosen ?? undefined,
        worktreeId: scopedWorktree.worktreeId,
        ...options,
      });
      if (outcome === "created") release();
      if (outcome === "picker") setHeldPrompt({ release });
    },
  };
}
