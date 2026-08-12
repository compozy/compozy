import { useSessionClearDialog } from "@/hooks/routes/use-session-clear-dialog";
import { useSessionDeleteDialog } from "@/hooks/routes/use-session-delete-dialog";
import { useSessionPageControls } from "@/hooks/routes/use-session-page-controls";

import { useSessionWindowSidebar } from "./use-session-window-sidebar";
import {
  getSessionPromptRuntimeSnapshot,
  type InspectorMemoryState,
  type InspectorUsage,
  SessionGoalHeadAction,
  type SessionPayload,
  useSessionCommands,
  useSessionGoalHeader,
  useSessionInspectorState,
  useSessionLedger,
  useSessionPromptRuntimeContext,
  useSessionTopbarSlot,
  useSessionUsage,
} from "@/systems/session";
import { useSessionVaultSecrets } from "@/systems/vault";

export function useSessionWindowController(input: {
  windowId: string;
  sessionId: string;
  workspaceId: string;
  session: SessionPayload;
  onDeleteSuccess: () => void;
  liveDataEnabled: boolean;
}) {
  const { windowId, sessionId, workspaceId, session, onDeleteSuccess, liveDataEnabled } = input;
  const promptRuntime = useSessionPromptRuntimeContext();
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
    error: sessionLedger.error,
  };
  const sessionUsage = useSessionUsage(sessionId, session.workspace_id, session.state, {
    enabled: inspectorEnabled,
  });
  const sessionCommands = useSessionCommands(workspaceId, sessionId);
  const usage = sessionUsage.data;
  const inspectorUsage: InspectorUsage | null = usage
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
  const deleteDialog = useSessionDeleteDialog(controls.handleDelete);
  const clearDialog = useSessionClearDialog(controls.handleClear);
  const sidebar = useSessionWindowSidebar({ windowId, workspaceId, sessionId });
  // Secondary goal reader for the head action — the goal strip inside the
  // thread owns the loop-stream reconciliation, so this instance reads cache only.
  const goal = useSessionGoalHeader(workspaceId, sessionId, {
    enabled: liveDataEnabled,
    stream: false,
  });

  useSessionTopbarSlot({
    session,
    isDeleting: controls.isDeleting,
    isStopping: controls.isStopping,
    isResuming: controls.isResuming,
    isUnarchiving: controls.isUnarchiving,
    isClearing: controls.isClearing,
    canClear: controls.canClear,
    inspectorOpen: inspector.open,
    sidebarOpen: sidebar.open,
    onSidebarToggle: sidebar.toggle,
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
    clearDialog,
    commandCatalog: sessionCommands.catalog,
    commandCatalogStatus: sessionCommands.isPending ? ("loading" as const) : ("ready" as const),
    refreshCommandCatalog: () => {
      void sessionCommands.refetch();
    },
  };
}
