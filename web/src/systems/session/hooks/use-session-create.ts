import { use } from "react";
import { useSelector } from "@xstate/store-react";
import { toast } from "@compozy/ui";

import type { TerminalQuote } from "@/systems/terminal/parts";
import { parseTerminalQuote } from "@/systems/terminal/parts";

import { SessionCreateContext } from "../contexts/session-create-context-value";
import { clearPendingTerminalQuote, holdPendingTerminalQuote } from "../lib/session-terminal-quote";
import type { SessionCreateOpenOptions } from "../lib/session-create-target";
import type { SessionCreateStore } from "../stores/session-create-store";
import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { useActiveWorkspace, useScopedWorktreeFilter } from "@/systems/workspace";

export function useSessionCreateStore(): SessionCreateStore {
  const store = use(SessionCreateContext);
  if (!store) {
    throw new Error("useSessionCreateStore must be used inside <SessionCreateProvider>");
  }
  return store;
}

export function useSessionCreateActions() {
  const store = useSessionCreateStore();
  const {
    dataScope,
    registeredWorkspaces,
    runtimeWorkspace,
    runtimeWorkspaceId,
    setActiveWorkspaceId,
  } = useActiveWorkspace();
  // Every settled destination has a target: a project, User, or — under All
  // projects — the one picked inline (ADR-003).
  const canStart = dataScope !== null;
  const scopeId = useWorktreeScopeId();
  const scopedWorktree = useScopedWorktreeFilter(runtimeWorkspaceId, scopeId);
  const openForAgent = (agentName: string, options: SessionCreateOpenOptions = {}) => {
    if (!canStart) {
      toast.error("Select an active workspace before starting a session.");
      return;
    }
    if (!scopedWorktree.resolved) {
      toast.error("Worktrees are not available yet. Try again.");
      return;
    }
    if (isSessionCreateSubmitting(store)) return;
    clearPendingTerminalQuote();
    store.trigger.dialogOpened({
      agentName: agentName.trim() || runtimeWorkspace?.default_agent?.trim() || "",
      workspaceId: runtimeWorkspaceId ?? "",
      ...(options.folderId ? { folderId: options.folderId } : {}),
      ...(options.workspaceId || options.profile
        ? { target: { project: options.workspaceId, profile: options.profile } }
        : {}),
      ...(scopedWorktree.worktreeId
        ? { environment: { kind: "worktree", worktreeId: scopedWorktree.worktreeId } }
        : {}),
    });
  };

  const openCreateDialog = (pendingPrompt?: string) => {
    if (!canStart) {
      toast.error("Select an active workspace before starting a session.");
      return;
    }
    if (!scopedWorktree.resolved) {
      toast.error("Worktrees are not available yet. Try again.");
      return;
    }
    store.trigger.dialogOpened({
      agentName: runtimeWorkspace?.default_agent?.trim() ?? "",
      workspaceId: runtimeWorkspaceId ?? "",
      ...(pendingPrompt !== undefined ? { pendingPrompt } : {}),
      ...(scopedWorktree.worktreeId
        ? { environment: { kind: "worktree" as const, worktreeId: scopedWorktree.worktreeId } }
        : {}),
    });
  };

  const openWithPrompt = (firstMessage: string) => {
    if (isSessionCreateSubmitting(store)) return;
    const quote = parseTerminalQuote(firstMessage);
    if (quote) {
      holdPendingTerminalQuote(quote);
      openCreateDialog();
      return;
    }
    clearPendingTerminalQuote();
    openCreateDialog(firstMessage);
  };

  const openWithTerminalQuote = (quote: TerminalQuote) => {
    if (isSessionCreateSubmitting(store)) return;
    holdPendingTerminalQuote(quote);
    openCreateDialog();
  };

  /**
   * Starts session creation already pointed at one worktree, so the operator is
   * never asked to re-pick the environment they just came from.
   */
  const openForWorktree = (workspaceId: string, worktreeId: string) => {
    if (workspaceId.trim() === "") {
      toast.error("Choose a workspace before starting a session.");
      return;
    }
    if (isSessionCreateSubmitting(store)) return;
    clearPendingTerminalQuote();
    if (runtimeWorkspaceId !== workspaceId) {
      setActiveWorkspaceId(workspaceId);
    }
    store.trigger.dialogOpened({
      agentName:
        registeredWorkspaces
          .find(workspace => workspace.id === workspaceId)
          ?.default_agent?.trim() ?? "",
      workspaceId,
      environment: { kind: "worktree", worktreeId },
    });
  };

  return { openForAgent, openForWorktree, openWithPrompt, openWithTerminalQuote, canStart };
}

function isSessionCreateSubmitting(store: SessionCreateStore): boolean {
  return store.getSnapshot().context.operation.status === "submitting";
}

/**
 * A session can be started from here: a project is focused, or All projects
 * offers the inline Project picker. User focus joins once User sessions ship.
 */
export function useSessionCreateHasActiveWorkspace(): boolean {
  return useActiveWorkspace().dataScope !== null;
}

export function useSessionCreateIsCreating(): boolean {
  const store = useSessionCreateStore();
  return useSelector(store, snapshot => snapshot.context.operation.status === "submitting");
}

export function useSessionCreatePendingAgentName(): string | null {
  const store = useSessionCreateStore();
  return useSelector(store, snapshot => {
    const { operation } = snapshot.context;
    return operation.status === "submitting" ? operation.agentName : null;
  });
}
