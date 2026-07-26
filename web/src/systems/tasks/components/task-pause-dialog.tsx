import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Spinner,
  Textarea,
} from "@agh/ui";

import type { useTaskPauseDialog } from "../hooks/use-task-pause-dialog";

export interface TaskPauseDialogProps {
  dialog: ReturnType<typeof useTaskPauseDialog>;
  isPending?: boolean;
}

/** Pause confirmation with a required reason (recorded on the task record). */
export function TaskPauseDialog({ dialog, isPending = false }: TaskPauseDialogProps) {
  const handleOpenChange = (open: boolean) => {
    if (!open && isPending) return;
    dialog.onOpenChange(open);
  };

  return (
    <Dialog onOpenChange={handleOpenChange} open={dialog.isOpen}>
      <DialogContent
        className="max-w-md"
        data-testid="tasks-detail-pause-dialog"
        showCloseButton={!isPending}
      >
        <DialogHeader>
          <DialogTitle>Pause task?</DialogTitle>
          <DialogDescription>
            New scheduler claims stop for this task; active runs continue.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-2">
          <label className="eyebrow text-muted" htmlFor="tasks-detail-pause-reason">
            Reason
          </label>
          <Textarea
            aria-describedby={dialog.error ? "tasks-detail-pause-error" : undefined}
            aria-invalid={Boolean(dialog.error)}
            data-testid="tasks-detail-pause-reason"
            disabled={isPending}
            id="tasks-detail-pause-reason"
            onChange={dialog.onReasonChange}
            rows={3}
            value={dialog.reason}
          />
          {dialog.error ? (
            <p
              className="text-form-hint text-danger"
              data-testid="tasks-detail-pause-error"
              id="tasks-detail-pause-error"
              role="alert"
            >
              {dialog.error}
            </p>
          ) : null}
        </div>
        <DialogFooter className="gap-2">
          <Button
            aria-busy={isPending || undefined}
            className="min-h-6"
            disabled={isPending}
            onClick={dialog.close}
            size="sm"
            type="button"
            variant="neutral"
          >
            Cancel
          </Button>
          <Button
            aria-busy={isPending || undefined}
            className="min-h-6"
            data-testid="tasks-detail-pause-confirm"
            disabled={isPending}
            onClick={() => void dialog.confirm()}
            size="sm"
            type="button"
          >
            {isPending ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {isPending ? "Pausing…" : "Pause task"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
