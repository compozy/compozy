import { useState, type FormEvent } from "react";
import { Clock } from "lucide-react";

import {
  Button,
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
  Input,
  Spinner,
} from "@compozy/ui";

import type { SessionLaterConfirmation } from "../hooks/use-session-later-actions";
import type { MoveSessionToLaterRequest } from "../adapters/session-later-api";
import {
  laterInputValue,
  laterReasonError,
  laterReturnAtFromInput,
  SESSION_LATER_REASON_MAX,
} from "../lib/session-later";
import { getSessionDisplayTitle } from "../lib/session-display-title";

export type SessionLaterDialogProps = SessionLaterConfirmation;

function laterDialogTitle(count: number): string {
  return count === 1 ? "Move to Later" : `Move ${count} sessions to Later`;
}

/** Parking stops the tree, so the dialog says so before the operator confirms. */
function laterDialogDescription(count: number, title: string | null): string {
  if (count === 1) {
    const subject = title ? `“${title}”` : "This session";
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
  const [returnInput, setReturnInput] = useState("");
  const [submitted, setSubmitted] = useState(false);
  const [min] = useState(() => laterInputValue(new Date()));
  const reasonError = laterReasonError(reason);
  const returnAt = laterReturnAtFromInput(returnInput);
  // The return date is checked on submit: a half-typed date is not an error yet.
  const returnError = submitted ? returnAt.error : null;

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitted(true);
    if (isMoving || reasonError || returnAt.error) return;
    const trimmed = reason.trim();
    onConfirm({
      ...(trimmed ? { reason: trimmed } : {}),
      ...(returnAt.returnAt ? { return_at: returnAt.returnAt } : {}),
    });
  };

  return (
    <form className="flex flex-col gap-4" noValidate onSubmit={submit}>
      <DialogHeader>
        <DialogTitle>{laterDialogTitle(count)}</DialogTitle>
        <DialogDescription>{laterDialogDescription(count, title)}</DialogDescription>
      </DialogHeader>
      <Field data-invalid={Boolean(reasonError)}>
        <FieldLabel htmlFor="session-later-reason">Reason</FieldLabel>
        <Input
          aria-invalid={Boolean(reasonError)}
          autoFocus
          data-testid="session-later-reason"
          disabled={isMoving}
          id="session-later-reason"
          maxLength={SESSION_LATER_REASON_MAX}
          onChange={event => setReason(event.target.value)}
          placeholder="Optional"
          value={reason}
        />
        {reasonError ? <FieldError>{reasonError}</FieldError> : null}
      </Field>
      <Field data-invalid={Boolean(returnError)}>
        <FieldLabel htmlFor="session-later-return">Remind me on</FieldLabel>
        <Input
          aria-invalid={Boolean(returnError)}
          className="w-auto"
          data-testid="session-later-return"
          disabled={isMoving}
          id="session-later-return"
          min={min}
          onChange={event => setReturnInput(event.target.value)}
          type="datetime-local"
          value={returnInput}
        />
        {returnError ? (
          <FieldError data-testid="session-later-return-error">{returnError}</FieldError>
        ) : (
          <FieldDescription>Optional. On this date it comes back to Needs you.</FieldDescription>
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
              <Clock className="size-3" />
              Move to Later
            </>
          )}
        </Button>
      </DialogFooter>
    </form>
  );
}

/**
 * Move to Later for one session or a selection: an optional one-line reason and
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
        className="max-w-md"
        data-testid="session-later-dialog"
        showCloseButton={!isMoving}
      >
        <SessionLaterForm
          count={sessions.length}
          isMoving={isMoving}
          key={`${sessions.map(session => session.id).join(",")}:${open ? "open" : "closed"}`}
          onCancel={() => onOpenChange(false)}
          onConfirm={onConfirm}
          title={only ? getSessionDisplayTitle(only) : null}
        />
      </DialogContent>
    </Dialog>
  );
}
