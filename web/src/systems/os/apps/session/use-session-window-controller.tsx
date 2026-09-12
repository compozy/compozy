import { useSessionClearDialog } from "@/hooks/routes/use-session-clear-dialog";
import { useSessionDeleteDialog } from "@/hooks/routes/use-session-delete-dialog";
import { useSessionRenameDialog } from "@/hooks/routes/use-session-rename-dialog";
import { useSessionPageControls } from "@/hooks/routes/use-session-page-controls";
import { shallowEqual } from "@xstate/store";
import { useSelector } from "@xstate/store-react";

import { useSessionWindowSidebar } from "./use-session-window-sidebar";
import {
  getSessionPromptRuntimeSnapshot,
  type InspectorUsage,
  isSessionTransportDisconnected,
  SessionGoalHeadAction,
  type SessionPayload,
  SessionTransportChip,
  useSessionCommands,
  useSessionGoalHeader,
  useSessionInspectorState,
  useSessionPromptRuntimeContext,
  useSessionTopbarSlot,
  useSessionTransportState,
  useSessionWorktreeBinding,
  useSessionContext,
  useSessionUsageTurns,
} from "@/systems/session";
import type { WorktreePayload } from "@/systems/workspace";

function toInspectorUsage(
  usage: ReturnType<typeof useSessionContext>["usage"]
): InspectorUsage | null {
  return usage
    ? {
        tokensIn: usage.input_tokens ?? undefined,
        cacheReadTokens: usage.cache_read_tokens ?? undefined,
        cacheWriteTokens: usage.cache_write_tokens ?? undefined,
        tokensOut: usage.output_tokens ?? undefined,
        totalTokens: usage.total_tokens ?? undefined,
        costUsd: usage.total_cost ?? undefined,
        costCurrency: usage.cost_currency || undefined,
        costStatus: usage.cost_status ?? undefined,
        costSource: usage.cost_source ?? undefined,
        turnCount: usage.turn_count,
      }
    : null;
}

/** Coordinates session window reads and projects query state into inspector view models. */
export function useSessionWindowController(input: {
  windowId: string;
  sessionId: string;
  workspaceId: string;
  session: SessionPayload;
  onDeleteSuccess: () => void;
  liveDataEnabled: boolean;
  onOpenWorktreeContext?: (workspaceId: string, worktree: WorktreePayload) => void;
  onResolveMissingWorktree?: (workspaceId: string, worktree: WorktreePayload) => void;
}) {
  const {
    windowId,
    sessionId,
    workspaceId,
    session,
    onDeleteSuccess,
    liveDataEnabled,
    onOpenWorktreeContext,
    onResolveMissingWorktree,
  } = input;
  const promptRuntime = useSessionPromptRuntimeContext();
  const promptRuntimeSnapshot = useSelector(
    promptRuntime,
    () => getSessionPromptRuntimeSnapshot(promptRuntime),
    shallowEqual
  );
  const controls = useSessionPageControls(sessionId, session, {
    getRuntimeSnapshot: () => getSessionPromptRuntimeSnapshot(promptRuntime),
    onDeleteSuccess,
    workspaceId: session.workspace_id,
  });
  const inspector = useSessionInspectorState(sessionId);
  const usageEnabled = liveDataEnabled || inspector.open;
  const sessionContext = useSessionContext(sessionId, workspaceId, session.state, {
    enabled: usageEnabled,
  });
  const sessionUsageTurns = useSessionUsageTurns(sessionId, workspaceId, session.state, {
    enabled: usageEnabled,
  });
  const sessionCommands = useSessionCommands(workspaceId, sessionId, { enabled: liveDataEnabled });
  const inspectorUsage = toInspectorUsage(sessionContext.usage);
  const deleteDialog = useSessionDeleteDialog(controls.handleDelete);
  const renameDialog = useSessionRenameDialog(controls.handleRename);
  const clearDialog = useSessionClearDialog(controls.handleClear);
  const transport = useSessionTransportState();
  const sidebar = useSessionWindowSidebar({
    sessionId,
    // The list never reads "connected" under a dead stream (US-018.AC-1).
    transportDisconnected:
      transport.phase === "failed" || isSessionTransportDisconnected(transport),
    windowId,
    workspaceId,
  });
  // Secondary goal reader for the head action — the goal strip inside the
  // thread owns the loop-stream reconciliation, so this instance reads cache only.
  const goal = useSessionGoalHeader(workspaceId, sessionId, {
    enabled: liveDataEnabled,
    stream: false,
  });

  const worktreeBinding = useSessionWorktreeBinding({
    workspaceId,
    worktreeId: session.worktree_id,
    commandCatalog: sessionCommands.catalog,
    enabled: liveDataEnabled,
  });

  useSessionTopbarSlot({
    session,
    worktreeBinding: worktreeBinding.bound
      ? {
          worktreeId: worktreeBinding.worktreeId,
          worktree: worktreeBinding.worktree,
          onOpenContext:
            worktreeBinding.worktree && onOpenWorktreeContext
              ? () => onOpenWorktreeContext(workspaceId, worktreeBinding.worktree!)
              : undefined,
          onResolve:
            worktreeBinding.worktree && onResolveMissingWorktree
              ? () => onResolveMissingWorktree(workspaceId, worktreeBinding.worktree!)
              : undefined,
        }
      : undefined,
    isDeleting: controls.isDeleting,
    isRenaming: controls.isRenaming,
    isStopping: controls.isStopping,
    isResuming: controls.isResuming,
    isUnarchiving: controls.isUnarchiving,
    isClearing: controls.isClearing,
    canClear: controls.canClear,
    inspectorOpen: inspector.open,
    sidebarOpen: sidebar.open,
    onSidebarToggle: sidebar.toggle,
    transportChip: <SessionTransportChip windowLive={liveDataEnabled} />,
    goalAction: (
      <SessionGoalHeadAction
        snapshot={goal.snapshot}
        pendingAction={goal.pendingAction}
        onPause={goal.onPause}
        onResume={goal.onResume}
        onApprove={goal.onApprove}
        onClear={goal.onClear}
      />
    ),
    onInspectorToggle: inspector.toggle,
    onDelete: deleteDialog.openDialog,
    onRename: renameDialog.openDialog,
    onStop: controls.handleStop,
    onResume: controls.handleResume,
    onUnarchive: controls.handleUnarchive,
    onClear: clearDialog.openDialog,
  });

  return {
    controls,
    inspector,
    sidebar,
    sessionContext,
    sessionUsageTurns,
    inspectorUsage,
    deleteDialog,
    renameDialog,
    clearDialog,
    commandCatalog: sessionCommands.catalog,
    commandCatalogStatus: sessionCommands.isPending ? ("loading" as const) : ("ready" as const),
    refreshCommandCatalog: () => {
      void sessionCommands.refetch();
    },
    worktreeBinding,
    promptRuntimeSnapshot,
    activityGoal: goal.snapshot,
  };
}
