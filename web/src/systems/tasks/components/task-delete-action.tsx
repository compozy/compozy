import { Trash2 } from "lucide-react";
import { useState, type ComponentProps } from "react";

import { Button, ConfirmDialog, DialogTrigger, Spinner } from "@agh/ui";

type ButtonSize = ComponentProps<typeof Button>["size"];
type ButtonVariant = ComponentProps<typeof Button>["variant"];

export interface TaskDeleteActionProps {
  taskId: string;
  taskTitle: string;
  onDelete: (taskId: string) => void;
  isPending?: boolean;
  size?: ButtonSize;
  triggerLabel?: string;
  triggerVariant?: ButtonVariant;
  triggerTestId?: string;
  dialogTestId?: string;
  cancelTestId?: string;
  confirmTestId?: string;
  /** Controlled open — use with `hideTrigger` when opened from a menu item. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  /** Omit the built-in DialogTrigger (caller owns the open control). */
  hideTrigger?: boolean;
}

export function TaskDeleteAction({
  taskId,
  taskTitle,
  onDelete,
  isPending = false,
  size = "sm",
  triggerLabel = "Delete",
  triggerVariant = "ghost",
  triggerTestId = "task-delete-trigger",
  dialogTestId = "task-delete-dialog",
  cancelTestId = "task-delete-cancel",
  confirmTestId = "task-delete-confirm",
  open: openProp,
  onOpenChange,
  hideTrigger = false,
}: TaskDeleteActionProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = openProp !== undefined;
  const open = controlled ? openProp : uncontrolledOpen;

  const setOpen = (next: boolean) => {
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };

  const handleConfirm = () => {
    setOpen(false);
    onDelete(taskId);
  };

  return (
    <ConfirmDialog
      cancelButtonProps={{ "data-testid": cancelTestId, disabled: isPending }}
      cancelLabel="Cancel"
      confirmButtonProps={{ "data-testid": confirmTestId }}
      confirmIcon={isPending ? Spinner : Trash2}
      confirmLabel={isPending ? "Deleting" : "Delete task"}
      contentProps={{ "data-testid": dialogTestId, showCloseButton: !isPending }}
      description={
        <>
          This permanently removes <strong>{taskTitle}</strong> and its stored runs, events, and
          triage state. Delete is blocked while the task still has child tasks or active runs.
        </>
      }
      isPending={isPending}
      onConfirm={handleConfirm}
      onOpenChange={setOpen}
      open={open}
      title="Delete task?"
      tone="danger"
    >
      {hideTrigger ? null : (
        <DialogTrigger
          render={
            <Button
              data-testid={triggerTestId}
              disabled={isPending}
              size={size}
              type="button"
              variant={triggerVariant}
            />
          }
        >
          {isPending ? <Spinner className="size-3" /> : <Trash2 className="size-3" />}
          {triggerLabel}
        </DialogTrigger>
      )}
    </ConfirmDialog>
  );
}
