import { ConfirmDialog } from "@compozy/ui";

export interface SessionWorkspaceSwitchDialogProps {
  open: boolean;
  /** Name of the workspace that owns the linked session. */
  workspaceName: string;
  /** True when the owner is the operator-home row — the link targets Global scope. */
  isGlobal?: boolean;
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
  isGlobal = false,
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
      title={isGlobal ? "Turn on Global scope?" : "Switch workspace?"}
      description={
        isGlobal
          ? "This session runs in Global scope (~). Confirming turns Global scope on — your workspace selection is remembered."
          : `This session belongs to ${workspaceName}. Switching changes the active workspace and the windows open in it.`
      }
      confirmLabel={isGlobal ? "Turn on Global scope" : "Switch workspace"}
      cancelLabel="Stay here"
      onConfirm={onConfirm}
      contentProps={{ "data-testid": "session-workspace-switch-dialog" }}
      confirmButtonProps={{ "data-testid": "session-workspace-switch-confirm" }}
      cancelButtonProps={{ "data-testid": "session-workspace-switch-cancel" }}
    />
  );
}
