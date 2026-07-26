import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Textarea,
} from "@agh/ui";

import type { useForceFailDialog } from "../hooks/use-force-fail-dialog";

export interface TaskRunForceFailDialogProps {
  dialog: ReturnType<typeof useForceFailDialog>;
  isPending?: boolean;
}

/** Force-fail confirmation with a required reason recorded in the audit log. */
export function TaskRunForceFailDialog({ dialog, isPending = false }: TaskRunForceFailDialogProps) {
  return (
    <Dialog onOpenChange={dialog.handleOpenChange} open={dialog.isOpen}>
      <DialogContent
        className="max-w-md"
        data-testid="tasks-run-force-fail-dialog"
        showCloseButton={!isPending}
      >
        <DialogHeader>
          <DialogTitle>Force fail this run?</DialogTitle>
          <DialogDescription>
            The run is marked failed immediately. The reason is recorded in the audit log.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-2">
          <label className="eyebrow text-muted" htmlFor={dialog.reasonId}>
            Reason
          </label>
          <Textarea
            aria-describedby={dialog.reasonHintId}
            aria-invalid={Boolean(dialog.error)}
            data-testid="tasks-run-force-fail-reason"
            disabled={isPending}
            id={dialog.reasonId}
            onChange={event => dialog.changeReason(event.target.value)}
            rows={3}
            value={dialog.reason}
          />
          {dialog.error ? (
            <p className="text-form-hint text-danger" id={dialog.reasonHintId} role="alert">
              {dialog.error}
            </p>
          ) : null}
        </div>
        <DialogFooter className="gap-2">
          <Button
            disabled={isPending}
            onClick={() => dialog.handleOpenChange(false)}
            size="sm"
            type="button"
            variant="neutral"
          >
            Cancel
          </Button>
          <Button
            data-testid="tasks-run-force-fail-confirm"
            disabled={isPending}
            onClick={() => void dialog.confirm()}
            size="sm"
            type="button"
            variant="destructive"
          >
            Force fail run
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
