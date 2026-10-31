import { useRef, useState, type FormEvent } from "react";
import { Bookmark } from "@compozy/ui/icons";

import {
  Button,
  DateTimePicker,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  MarkdownEditor,
  Spinner,
} from "@compozy/ui";

import type { SessionLaterConfirmation } from "../hooks/use-session-later-actions";
import type { MoveSessionToLaterRequest } from "../adapters/session-later-api";
import { markdownLineToPlainText } from "../lib/markdown-plain-text";
import { laterReasonError, laterReturnAtFromDate, laterReturnPresets } from "../lib/session-later";
import { getSessionDisplayTitle } from "../lib/session-display-title";

export type SessionLaterDialogProps = SessionLaterConfirmation;

function laterDialogTitle(count: number): string {
  return count === 1 ? "Move to Later" : `Move ${count} sessions to Later`;
}

/** Parking stops the tree, so the dialog says so before the operator confirms. */
function laterDialogDescription(count: number, title: string | null): string {
  if (count === 1) {
    // Titles can carry Markdown from the agent; the quote reads it as plain text.
    const plain = title ? markdownLineToPlainText(title) : "";
    const subject = plain ? `“${plain}”` : "This session";
    return `${subject} stops, with everything it runs, and stays out of your lists until you bring it back.`;
  }
  return "These sessions stop, with everything they run, and stay out of your lists until you bring them back.";
}

function SessionLaterForm({
  count,
  title,
  isMoving,
  onCancel,
  onConfirm,
}: {
  count: number;
  /** The one session's title; absent for a bulk move. */
  title: string | null;
  isMoving: boolean;
  onCancel: () => void;
  onConfirm: (request: MoveSessionToLaterRequest) => void;
}) {
  const [reason, setReason] = useState("");
  const [returnDate, setReturnDate] = useState<Date | null>(null);
  const [submitted, setSubmitted] = useState(false);
  // The earliest pick and the suggestions are fixed when the dialog opens.
  const [{ min, presets }] = useState(() => ({ min: new Date(), presets: laterReturnPresets() }));
  const formRef = useRef<HTMLFormElement>(null);
  const reasonError = laterReasonError(reason);
  const returnAt = laterReturnAtFromDate(returnDate);
  // The return date is checked on submit: a picked time can pass while the dialog is open.
  const returnError = submitted ? returnAt.error : null;

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitted(true);
    // Read the clock now, not at the last render: the picked time may have passed.
    const checked = laterReturnAtFromDate(returnDate);
    if (isMoving || reasonError || checked.error) return;
    const trimmed = reason.trim();
    onConfirm({
      ...(trimmed ? { reason: trimmed } : {}),
      ...(checked.returnAt ? { return_at: checked.returnAt } : {}),
    });
  };

  return (
    <form className="flex flex-col gap-4" noValidate onSubmit={submit} ref={formRef}>
      <DialogHeader>
        <DialogTitle>{laterDialogTitle(count)}</DialogTitle>
        <DialogDescription>{laterDialogDescription(count, title)}</DialogDescription>
      </DialogHeader>
      <Field data-invalid={Boolean(reasonError)}>
        <FieldLabel id="session-later-reason-label">Reason</FieldLabel>
        <MarkdownEditor
          aria-describedby={reasonError ? "session-later-reason-error" : undefined}
          aria-invalid={Boolean(reasonError)}
          aria-labelledby="session-later-reason-label"
          autoFocus
          data-testid="session-later-reason"
          disabled={isMoving}
          id="session-later-reason"
          onSubmit={() => formRef.current?.requestSubmit()}
          onValueChange={setReason}
          placeholder="Optional. Why it waits, what to pick up next."
          value={reason}
        />
        {reasonError ? (
          <FieldError id="session-later-reason-error">{reasonError}</FieldError>
        ) : null}
      </Field>
      <Field data-invalid={Boolean(returnError)}>
        <FieldLabel id="session-later-return-label">Remind me on</FieldLabel>
        <DateTimePicker
          aria-describedby="session-later-return-hint"
          aria-invalid={Boolean(returnError)}
          aria-labelledby="session-later-return-label"
          data-testid="session-later-return"
          disabled={isMoving}
          id="session-later-return"
          min={min}
          onValueChange={setReturnDate}
          placeholder="Pick a date"
          presets={presets}
          value={returnDate}
        />
        {returnError ? (
          <FieldError data-testid="session-later-return-error" id="session-later-return-hint">
            {returnError}
          </FieldError>
        ) : (
          <FieldDescription id="session-later-return-hint">
            Optional. On this date it comes back to Needs you.
          </FieldDescription>
        )}
      </Field>
      <DialogFooter className="gap-2">
        <Button
          data-testid="session-later-cancel"
          disabled={isMoving}
          onClick={onCancel}
          type="button"
          variant="ghost"
        >
          Cancel
        </Button>
        <Button
          data-testid="session-later-confirm"
          disabled={isMoving || Boolean(reasonError)}
          type="submit"
        >
          {isMoving ? (
            <>
              <Spinner className="size-3" />
              Moving to Later
            </>
          ) : (
            <>
              <Bookmark className="size-3" />
              Move to Later
            </>
          )}
        </Button>
      </DialogFooter>
    </form>
  );
}

/**
 * Move to Later for one session or a selection: an optional Markdown reason and
 * an optional return date. Parking stops the session tree first, so the dialog
 * stays open with a pending confirm until the daemon answers.
 */
export function SessionLaterDialog({
  open,
  onOpenChange,
  sessions,
  isMoving,
  onConfirm,
}: SessionLaterDialogProps) {
  const only = sessions.length === 1 ? sessions[0] : undefined;
  return (
    <Dialog
      open={open}
      onOpenChange={nextOpen => {
        if (isMoving && !nextOpen) return;
        onOpenChange(nextOpen);
      }}
    >
      <DialogContent
        className="max-w-lg"
        data-testid="session-later-dialog"
        showCloseButton={!isMoving}
      >
        <SessionLaterForm
          count={sessions.length}
          isMoving={isMoving}
          key={open ? "open" : "closed"}
          onCancel={() => onOpenChange(false)}
          onConfirm={onConfirm}
          title={only ? getSessionDisplayTitle(only) : null}
        />
      </DialogContent>
    </Dialog>
  );
}
