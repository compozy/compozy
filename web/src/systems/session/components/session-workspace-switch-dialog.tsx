import { ConfirmDialog } from "@compozy/ui";

export interface SessionWorkspaceSwitchDialogProps {
  open: boolean;
  /** Name of the workspace that owns the linked session. */
  workspaceName: string;
  onConfirm: () => void;
  onCancel: () => void;
}

/**
 * Confirmation for a session deep link that belongs to another workspace (ADR-004).
 * Switching is a heavyweight context change — it swaps the active workspace and the windows
 * restored with it — so the operator confirms it explicitly instead of being moved silently.
 */
export function SessionWorkspaceSwitchDialog({
  open,
  workspaceName,
  onConfirm,
  onCancel,
}: SessionWorkspaceSwitchDialogProps) {
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={nextOpen => {
        if (!nextOpen) onCancel();
      }}
      tone="accent"
      title="Switch workspace?"
      description={`This session belongs to ${workspaceName}. Switching changes the active workspace and the windows open in it.`}
      confirmLabel="Switch workspace"
      cancelLabel="Stay here"
      onConfirm={onConfirm}
      contentProps={{ "data-testid": "session-workspace-switch-dialog" }}
      confirmButtonProps={{ "data-testid": "session-workspace-switch-confirm" }}
      cancelButtonProps={{ "data-testid": "session-workspace-switch-cancel" }}
    />
  );
}
