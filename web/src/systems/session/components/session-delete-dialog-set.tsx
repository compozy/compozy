import { Check, Info, RotateCcw, Trash2, X } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Spinner,
} from "@compozy/ui";

import type { SessionPayload } from "../types";
import type { SessionBatchResult } from "../lib/session-batch";
import { sessionSelectionCounts } from "../hooks/use-session-selection";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import { sessionBadgeSignal } from "../lib/session-badge";
import { sessionBadgeWordClass } from "../lib/session-badge-classes";
import { SessionBadgeMark } from "./session-badge-mark";

export interface SessionDeleteDialogSetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sessions: readonly SessionPayload[];
  results?: readonly SessionBatchResult[];
  isDeleting: boolean;
  onConfirm: () => void;
  onRetry?: () => void;
}

/** Plural confirmation preserves the original set while individual mutations settle. */
export function SessionDeleteDialogSet({
  open,
  onOpenChange,
  sessions,
  results = [],
  isDeleting,
  onConfirm,
  onRetry,
}: SessionDeleteDialogSetProps) {
  const count = sessions.length;
  const failed = results.filter(result => result.status === "failed").length;
  const deleted = results.filter(result => result.status === "done").length;
  const partial = !isDeleting && failed > 0;
  const active = sessionSelectionCounts(sessions, []).stoppable;
  const progressIndex = results.findIndex(result => result.status === "running");
  const note = isDeleting
    ? "Don't close the window while this runs."
    : partial || active === 0
      ? null
      : active === 1
        ? "1 of them is active."
        : `${active} of them are active.`;
  const resultsById = new Map(results.map(result => [result.id, result]));
  return (
    <Dialog
      open={open}
      onOpenChange={next => {
        if (!isDeleting || next) onOpenChange(next);
      }}
    >
      <DialogContent
        showCloseButton={!isDeleting}
        className="max-w-md"
        data-testid="delete-dialog"
        aria-busy={isDeleting}
      >
        <DialogHeader>
          <DialogTitle>Delete {count} sessions</DialogTitle>
          <DialogDescription aria-live="polite">
            {partial ? (
              <>
                <strong>
                  {deleted} deleted · {failed} couldn't be deleted.
                </strong>{" "}
                {failed === 1
                  ? "The one that failed is still in the list and still selected."
                  : "The ones that failed are still in the list and still selected."}
              </>
            ) : (
              <>
                This permanently removes <strong>{count} sessions</strong>, including their
                transcripts and history, and removes them from the session list.
              </>
            )}
          </DialogDescription>
        </DialogHeader>
        <ul className="overflow-hidden rounded-md border border-line bg-canvas">
          {sessions.slice(0, 5).map(session => {
            const result = resultsById.get(session.id);
            return (
              <li
                key={session.id}
                data-testid={`delete-dialog-row-${session.id}`}
                className="grid grid-cols-[8px_minmax(0,1fr)_auto_14px] items-center gap-x-2.5 border-t border-line-soft px-2.5 py-1.5 text-form first:border-t-0"
              >
                <SessionBadgeMark badge={session.badge} />
                <span className="truncate">{getSessionDisplayTitle(session)}</span>
                <span className={`font-mono text-micro ${sessionBadgeWordClass(session.badge)}`}>
                  {sessionBadgeSignal(session.badge).label}
                </span>
                <span className="grid size-3.5 place-items-center text-subtle">
                  {result?.status === "done" ? (
                    <Check className="size-3" aria-label="Deleted" />
                  ) : result?.status === "running" ? (
                    <Spinner className="size-3 motion-reduce:animate-none" aria-label="Deleting" />
                  ) : result?.status === "failed" ? (
                    <X className="size-3 text-danger" aria-label="Failed" />
                  ) : null}
                </span>
                {result?.status === "failed" ? (
                  <span className="col-start-2 col-end-5 text-micro text-danger">
                    Couldn't delete: {result.error}
                  </span>
                ) : null}
              </li>
            );
          })}
          {count > 5 ? (
            <li className="border-t border-line-soft px-2.5 py-1.5 text-micro text-subtle">
              and {count - 5} more
            </li>
          ) : null}
        </ul>
        <DialogFooter className="gap-2">
          {note ? (
            <span
              className="mr-auto flex items-center gap-1.5 text-micro text-muted"
              data-testid="delete-dialog-note"
            >
              {!isDeleting ? <Info className="size-3 shrink-0" /> : null}
              {note}
            </span>
          ) : null}
          <Button
            variant="ghost"
            disabled={isDeleting}
            onClick={() => onOpenChange(false)}
            data-testid="delete-dialog-cancel"
          >
            {partial ? "Close" : "Cancel"}
          </Button>
          <Button
            variant="destructive"
            disabled={isDeleting}
            onClick={partial ? onRetry : onConfirm}
            data-testid={partial ? "delete-dialog-retry" : "delete-dialog-confirm"}
          >
            {isDeleting ? (
              <>
                <Spinner className="size-3 motion-reduce:animate-none" />
                Deleting {Math.max(1, progressIndex + 1)} of {count}
              </>
            ) : partial ? (
              <>
                <RotateCcw className="size-3" />
                Retry {failed}
              </>
            ) : (
              <>
                <Trash2 className="size-3" />
                Delete {count} sessions
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
