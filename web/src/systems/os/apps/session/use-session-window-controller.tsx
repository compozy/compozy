import { useSessionClearDialog } from "@/hooks/routes/use-session-clear-dialog";
import { useSessionDeleteDialog } from "@/hooks/routes/use-session-delete-dialog";
import { useSessionRenameDialog } from "@/hooks/routes/use-session-rename-dialog";
import { useSessionPageControls } from "@/hooks/routes/use-session-page-controls";
import { shallowEqual } from "@xstate/store";
import { useSelector } from "@xstate/store-react";

import { useSessionWindowSidebar } from "./use-session-window-sidebar";
import {
  getSessionPromptRuntimeSnapshot,
  type InspectorMemoryState,
  type InspectorUsage,
  isSessionTransportDisconnected,
  SessionGoalHeadAction,
  type SessionPayload,
  SessionTransportChip,
  useSessionCommands,
  useSessionGoalHeader,
  useSessionInspectorState,
  useSessionLedger,
  useSessionPromptRuntimeContext,
  useSessionTopbarSlot,
  useSessionTransportState,
  useSessionWorktreeBinding,
  useSessionUsage,
} from "@/systems/session";
import { useSessionVaultSecrets } from "@/systems/vault";
import type { WorktreePayload } from "@/systems/workspace";

function toInspectorUsage(
  usage: ReturnType<typeof useSessionUsage>["data"]
): InspectorUsage | null {
  return usage
    ? {
        tokensIn: usage.input_tokens ?? undefined,
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
  const inspectorEnabled = inspector.open && liveDataEnabled;
  const sessionVault = useSessionVaultSecrets(sessionId, { enabled: inspectorEnabled });
  const sessionLedger = useSessionLedger(sessionId, session.workspace_id, {
    enabled: inspectorEnabled && session.state === "stopped",
  });
  const inspectorMemory: InspectorMemoryState = {
    ledger: sessionLedger.data ?? null,
    isLoading: sessionLedger.isLoading,
    availability: sessionLedger.availability,
    error: sessionLedger.availability ? null : sessionLedger.error,
  };
  const sessionUsage = useSessionUsage(sessionId, session.workspace_id, session.state, {
    enabled: inspectorEnabled,
  });
  const sessionCommands = useSessionCommands(workspaceId, sessionId, { enabled: liveDataEnabled });
  const inspectorUsage = toInspectorUsage(sessionUsage.data);
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
    inspectorMemory,
    inspectorUsage,
    sessionVault,
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
  };
}
