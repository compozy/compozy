import { Check, Trash2, X } from "lucide-react";

import { Alert, AlertAction, AlertDescription, Button, ConfirmDialog } from "@agh/ui";

import type { SandboxLastAction } from "../hooks/use-sandbox-page";
import type { SettingsSandboxEntry } from "@/systems/settings";

export function SandboxDeleteDialog({
  target,
  error,
  isDeleting,
  onClose,
  onConfirm,
}: {
  target: SettingsSandboxEntry | null;
  error: string | null;
  isDeleting: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const usage = target?.workspace_usage_count ?? 0;
  return (
    <ConfirmDialog
      open={Boolean(target)}
      title={target ? `Delete sandbox profile "${target.name}"?` : "Delete sandbox profile"}
      description={
        target
          ? "Removing the overlay stops making this profile selectable for new workspaces."
          : null
      }
      note={
        usage > 0 ? (
          <div className="flex flex-col gap-1" data-testid="sandbox-delete-usage">
            <span className="font-medium">
              {usage} {usage === 1 ? "workspace" : "workspaces"} currently reference this profile
            </span>
            <span>
              Existing sessions continue to run against their recorded profile. New sessions will
              fail to resolve this sandbox until another profile with the same name is added.
            </span>
          </div>
        ) : null
      }
      error={error}
      isPending={isDeleting}
      cancelLabel="Cancel"
      confirmLabel="Delete sandbox profile"
      confirmIcon={Trash2}
      contentProps={{ "data-testid": "settings-sandboxes-delete" }}
      noteProps={{ "data-testid": "settings-sandboxes-delete-fallback" }}
      errorProps={{ "data-testid": "settings-sandboxes-delete-error" }}
      cancelButtonProps={{
        "data-testid": "settings-sandboxes-delete-cancel",
        disabled: isDeleting,
      }}
      confirmButtonProps={{ "data-testid": "settings-sandboxes-delete-confirm" }}
      onConfirm={onConfirm}
      onOpenChange={next => {
        if (!next) onClose();
      }}
    />
  );
}

export function SandboxLastActionAlert({
  action,
  onDismiss,
}: {
  action: SandboxLastAction;
  onDismiss: () => void;
}) {
  const isSaved = action.kind === "saved";
  const restartBadge = action.result.restart_required
    ? "restart required to apply"
    : "applied immediately";
  const message = isSaved
    ? `Saved sandbox "${action.name}" · ${restartBadge}.`
    : action.usageCount > 0
      ? `Deleted "${action.name}" · ${action.usageCount} workspaces affected · ${restartBadge}.`
      : `Deleted "${action.name}" · ${restartBadge}.`;

  return (
    <Alert
      variant={isSaved ? "success" : "info"}
      role="status"
      data-testid="sandbox-page-action-result"
      data-kind={action.kind}
    >
      <Check className="size-3" />
      <AlertDescription className="text-xs">{message}</AlertDescription>
      <AlertAction>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={onDismiss}
          data-testid="sandbox-page-action-result-dismiss"
          aria-label="Dismiss result"
        >
          <X className="size-3" />
        </Button>
      </AlertAction>
    </Alert>
  );
}
