import { useSessionClearDialog } from "@/hooks/routes/use-session-clear-dialog";
import { useSessionDeleteDialog } from "@/hooks/routes/use-session-delete-dialog";
import { useSessionPageControls } from "@/hooks/routes/use-session-page-controls";
import {
  useSessionInspectorState,
  useSessionLedger,
  useSessionTopbarSlot,
  useSessionUsage,
  type InspectorMemoryState,
  type InspectorUsage,
  type SessionPayload,
} from "@/systems/session";
import { useSessionVaultSecrets } from "@/systems/vault";

export function useSessionWindowController(input: {
  sessionId: string;
  session: SessionPayload;
  onDeleteSuccess: () => void;
}) {
  const { sessionId, session, onDeleteSuccess } = input;
  const controls = useSessionPageControls(sessionId, session, {
    onDeleteSuccess,
    workspaceId: session.workspace_id,
  });
  const sessionVault = useSessionVaultSecrets(sessionId);
  const sessionLedger = useSessionLedger(sessionId, session.workspace_id, {
    enabled: session.state === "stopped",
  });
  const inspectorMemory: InspectorMemoryState = {
    ledger: sessionLedger.data ?? null,
    isLoading: sessionLedger.isLoading,
    error: sessionLedger.error,
  };
  const sessionUsage = useSessionUsage(sessionId, session.workspace_id, session.state);
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
  const inspector = useSessionInspectorState(sessionId);

  useSessionTopbarSlot({
    session,
    isDeleting: controls.isDeleting,
    isStopping: controls.isStopping,
    isResuming: controls.isResuming,
    isClearing: controls.isClearing,
    canClear: controls.canClear,
    inspectorOpen: inspector.open,
    onInspectorToggle: inspector.toggle,
    onDelete: deleteDialog.openDialog,
    onStop: controls.handleStop,
    onResume: controls.handleResume,
    onClear: clearDialog.openDialog,
  });

  return {
    controls,
    inspector,
    inspectorMemory,
    inspectorUsage,
    sessionVault,
    deleteDialog,
    clearDialog,
  };
}
