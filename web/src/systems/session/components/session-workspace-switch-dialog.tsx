import { ConfirmDialog } from "@compozy/ui";

export interface SessionWorkspaceSwitchDialogProps {
  open: boolean;
  /** Name of the workspace that owns the linked session. */
  workspaceName: string;
  /** True when the session has no project — confirming focuses User. */
  isUser?: boolean;
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
  isUser = false,
  onConfirm,
  onCancel,
}: SessionWorkspaceSwitchDialogProps) {
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={nextOpen => {
        if (!nextOpen) onCancel();
      }}
      tone="neutral"
      title={isUser ? "Switch to User?" : "Switch project?"}
      description={
        isUser
          ? "This session runs in User, outside any project. Switching shows User work and the windows open there."
          : `This session belongs to ${workspaceName}. Switching changes the active project and the windows open in it.`
      }
      confirmLabel={isUser ? "Switch to User" : "Switch project"}
      cancelLabel="Stay here"
      onConfirm={onConfirm}
      contentProps={{ "data-testid": "session-workspace-switch-dialog" }}
      confirmButtonProps={{ "data-testid": "session-workspace-switch-confirm" }}
      cancelButtonProps={{ "data-testid": "session-workspace-switch-cancel" }}
    />
  );
}
