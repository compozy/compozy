import { useState } from "react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  FieldError,
  FieldLabel,
  Textarea,
} from "@compozy/ui";

interface SchedulerPauseDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  isPending: boolean;
  onPause?: (reason: string) => void | Promise<void>;
}

/** Asks for a pause reason before pausing the task queue; the draft survives close. */
export function SchedulerPauseDialog({
  open,
  onOpenChange,
  isPending,
  onPause,
}: SchedulerPauseDialogProps) {
  const [pauseReason, setPauseReason] = useState("");
  const [pauseError, setPauseError] = useState<string | null>(null);

  const handleOpenChange = (next: boolean) => {
    onOpenChange(next);
    if (!next) {
      setPauseError(null);
    }
  };

  const handleConfirm = async () => {
    const reason = pauseReason.trim();
    if (!reason) {
      setPauseError("Provide a pause reason.");
      return;
    }
    if (!onPause) {
      return;
    }
    try {
      await onPause(reason);
      setPauseReason("");
      setPauseError(null);
      onOpenChange(false);
    } catch (error) {
      setPauseError(error instanceof Error ? error.message : "Couldn't pause the task queue.");
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        data-testid="scheduler-controls-pause-dialog"
        showCloseButton={!isPending}
        className="max-w-md"
      >
        <DialogHeader>
          <DialogTitle>Pause the task queue?</DialogTitle>
          <DialogDescription>New work won&apos;t start; running work finishes.</DialogDescription>
        </DialogHeader>
        <Field data-invalid={Boolean(pauseError) || undefined}>
          <FieldLabel htmlFor="scheduler-controls-pause-reason">Reason</FieldLabel>
          <Textarea
            aria-describedby={pauseError ? "scheduler-controls-pause-error" : undefined}
            aria-invalid={Boolean(pauseError)}
            data-testid="scheduler-controls-pause-reason"
            disabled={isPending}
            id="scheduler-controls-pause-reason"
            onChange={event => {
              setPauseReason(event.target.value);
              setPauseError(null);
            }}
            rows={3}
            value={pauseReason}
          />
          {pauseError ? (
            <FieldError
              data-testid="scheduler-controls-pause-error"
              id="scheduler-controls-pause-error"
            >
              {pauseError}
            </FieldError>
          ) : null}
        </Field>
        <DialogFooter className="gap-2">
          <Button
            disabled={isPending}
            onClick={() => handleOpenChange(false)}
            type="button"
            variant="neutral"
          >
            Cancel
          </Button>
          <Button
            data-testid="scheduler-controls-pause-confirm"
            disabled={isPending}
            onClick={() => void handleConfirm()}
            type="button"
          >
            Pause
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
