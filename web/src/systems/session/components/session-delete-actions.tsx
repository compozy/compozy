import { RotateCcw, Trash2 } from "@compozy/ui/icons";

import { Button, Spinner } from "@compozy/ui";

import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import type { SessionProcessChoiceCopy } from "../lib/session-process-choice";

export interface SessionDeleteActionsProps {
  isDeleting: boolean;
  /** A failed attempt is on screen: Close and Retry replace Cancel and Delete. */
  retrying: boolean;
  /** Present while the targets have processes running; adds Keep processes running. */
  processChoice: SessionProcessChoiceCopy | null;
  deleteLabel: string;
  deletingLabel: string;
  retryLabel: string;
  onClose: () => void;
  onConfirm: (processes?: SessionProcessPolicy) => void;
  onRetry?: () => void;
}

/**
 * The delete dialogs' buttons: Cancel, Keep processes running while processes
 * run, and the destructive Delete (or Retry after a failure, which repeats the
 * answer already given).
 */
export function SessionDeleteActions({
  isDeleting,
  retrying,
  processChoice,
  deleteLabel,
  deletingLabel,
  retryLabel,
  onClose,
  onConfirm,
  onRetry,
}: SessionDeleteActionsProps) {
  const choice = retrying ? null : processChoice;
  let confirmContent = (
    <>
      <Trash2 className="size-3" />
      {choice ? choice.confirmLabel : deleteLabel}
    </>
  );
  if (isDeleting) {
    confirmContent = (
      <>
        <Spinner className="size-3 motion-reduce:animate-none" />
        {deletingLabel}
      </>
    );
  } else if (retrying) {
    confirmContent = (
      <>
        <RotateCcw className="size-3" />
        {retryLabel}
      </>
    );
  }
  return (
    <>
      <Button
        type="button"
        variant="ghost"
        onClick={onClose}
        disabled={isDeleting}
        data-testid="delete-dialog-cancel"
      >
        {retrying ? "Close" : "Cancel"}
      </Button>
      {choice ? (
        <Button
          type="button"
          variant="outline"
          onClick={() => onConfirm("keep")}
          disabled={isDeleting}
          data-testid="delete-dialog-keep-processes"
        >
          {choice.keepLabel}
        </Button>
      ) : null}
      <Button
        type="button"
        variant="destructive"
        onClick={retrying ? onRetry : () => onConfirm(choice ? "terminate" : undefined)}
        disabled={isDeleting}
        data-testid={retrying ? "delete-dialog-retry" : "delete-dialog-confirm"}
      >
        {confirmContent}
      </Button>
    </>
  );
}
