import { use } from "react";
import { useSelector } from "@xstate/store-react";
import { toast } from "sonner";

import type { TerminalQuote } from "@/systems/terminal/parts";
import { parseTerminalQuote } from "@/systems/terminal/parts";

import { SessionCreateContext } from "../contexts/session-create-context-value";
import { clearPendingTerminalQuote, holdPendingTerminalQuote } from "../lib/session-terminal-quote";
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
  const { runtimeWorkspaceId, scope } = useActiveWorkspace();
  const scopeId = useWorktreeScopeId();
  const scopedWorktree = useScopedWorktreeFilter(
    scope === "workspace" ? runtimeWorkspaceId : null,
    scopeId
  );
  const openForAgent = (agentName: string) => {
    if (runtimeWorkspaceId === null) {
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
      agentName,
      workspaceId: runtimeWorkspaceId,
      ...(scopedWorktree.worktreeId
        ? { environment: { kind: "worktree", worktreeId: scopedWorktree.worktreeId } }
        : {}),
    });
  };

  const openCreateDialog = (firstMessage?: string) => {
    if (runtimeWorkspaceId === null) {
      toast.error("Select an active workspace before starting a session.");
      return;
    }
    if (!scopedWorktree.resolved) {
      toast.error("Worktrees are not available yet. Try again.");
      return;
    }
    store.trigger.dialogOpened({
      agentName: "",
      workspaceId: runtimeWorkspaceId,
      ...(firstMessage !== undefined ? { firstMessage } : {}),
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
      openCreateDialog("");
      return;
    }
    clearPendingTerminalQuote();
    openCreateDialog(firstMessage);
  };

  const openWithTerminalQuote = (quote: TerminalQuote) => {
    if (isSessionCreateSubmitting(store)) return;
    holdPendingTerminalQuote(quote);
    openCreateDialog("");
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
    store.trigger.dialogOpened({
      agentName: "",
      workspaceId,
      environment: { kind: "worktree", worktreeId },
    });
  };

  return { openForAgent, openForWorktree, openWithPrompt, openWithTerminalQuote };
}

function isSessionCreateSubmitting(store: SessionCreateStore): boolean {
  return store.getSnapshot().context.operation.status === "submitting";
}

export function useSessionCreateHasActiveWorkspace(): boolean {
  return useActiveWorkspace().runtimeWorkspaceId !== null;
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
