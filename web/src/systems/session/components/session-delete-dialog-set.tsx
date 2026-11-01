import { Info } from "@compozy/ui/icons";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  StateGlyph,
} from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { SessionPayload } from "../types";
import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import { sessionProcessChoiceCopy } from "../lib/session-process-choice";
import { sessionsRunningProcesses } from "../lib/session-processes";
import type { SessionBatchResult } from "../lib/session-batch";
import type { SessionChildrenChoice } from "../lib/session-tree";
import { sessionSelectionCounts } from "../hooks/use-session-selection";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import { sessionBadgeSignal } from "../lib/session-badge";
import { sessionBadgeWordClass } from "../lib/session-badge-classes";
import { SessionBadgeMark } from "./session-badge-mark";
import { SessionChildrenCheckbox } from "./session-children-checkbox";
import { SessionDeleteActions } from "./session-delete-actions";
import { sessionWaitingOn } from "../lib/session-pull-request";

export interface SessionDeleteDialogSetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sessions: readonly SessionPayload[];
  results?: readonly SessionBatchResult[];
  isDeleting: boolean;
  childSessions?: SessionChildrenChoice;
  /** `processes` answers what happens to running processes; absent when none run. */
  onConfirm: (processes?: SessionProcessPolicy) => void;
  onRetry?: () => void;
}

/** Plural confirmation preserves the original set while individual mutations settle. */
export function SessionDeleteDialogSet({
  open,
  onOpenChange,
  sessions,
  results = [],
  isDeleting,
  childSessions,
  onConfirm,
  onRetry,
}: SessionDeleteDialogSetProps) {
  const count = sessions.length;
  const failed = results.filter(result => result.status === "failed").length;
  const deleted = results.filter(result => result.status === "done").length;
  const partial = !isDeleting && failed > 0;
  const active = sessionSelectionCounts(sessions, []).stoppable;
  const progressIndex = results.findIndex(result => result.status === "running");
  const note = sessionDeleteNote(isDeleting, partial, active);
  const resultsById = new Map(results.map(result => [result.id, result]));
  const processCount =
    sessionsRunningProcesses(sessions) + (childSessions?.includedProcessCount ?? 0);
  const processChoice =
    processCount > 0 ? sessionProcessChoiceCopy("delete", count, processCount) : null;
  return (
    <Dialog
      open={open}
      onOpenChange={next => {
        if (!isDeleting || next) onOpenChange(next);
      }}
    >
      <DialogContent
        showCloseButton={!isDeleting}
        className="flex max-h-[calc(100dvh-2rem)] max-w-md flex-col overflow-hidden"
        data-testid="delete-dialog"
        aria-busy={isDeleting}
      >
        <DialogHeader className="shrink-0">
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
                {processChoice ? ` ${processChoice.description}` : null}
              </>
            )}
          </DialogDescription>
        </DialogHeader>
        <div
          role="region"
          aria-label="Deletion results"
          tabIndex={0}
          className="min-h-0 space-y-4 overflow-y-auto"
        >
          <ul className="overflow-hidden rounded-lg bg-sunken py-1">
            {sessions.slice(0, 5).map(session => (
              <SessionDeleteResultRow
                key={session.id}
                session={session}
                result={resultsById.get(session.id)}
              />
            ))}
            {count > 5 ? (
              <li className="px-3 py-1.5 text-micro text-subtle">and {count - 5} more</li>
            ) : null}
          </ul>
          <SessionDeleteOverflowErrors sessions={sessions.slice(5)} resultsById={resultsById} />
        </div>
        {childSessions && !partial ? (
          <SessionChildrenCheckbox action="delete" choice={childSessions} disabled={isDeleting} />
        ) : null}
        <DialogFooter className="shrink-0 flex-wrap gap-2">
          {note ? (
            <span
              className="mr-auto flex items-center gap-1.5 text-micro text-muted-foreground"
              data-testid="delete-dialog-note"
            >
              {!isDeleting ? <Info className="size-3 shrink-0" /> : null}
              {note}
            </span>
          ) : null}
          <SessionDeleteActions
            childrenPending={childSessions?.included === true && childSessions.pending}
            isDeleting={isDeleting}
            retrying={partial}
            processChoice={processChoice}
            deleteLabel={`Delete ${count} sessions`}
            deletingLabel={`Deleting ${Math.max(1, progressIndex + 1)} of ${count}`}
            retryLabel={`Retry ${failed}`}
            onClose={() => onOpenChange(false)}
            onConfirm={onConfirm}
            onRetry={onRetry}
          />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** Present a target and its latest deletion result without changing original set order. */
function SessionDeleteResultRow({
  session,
  result,
  className,
  ...props
}: {
  session: SessionPayload;
  result?: SessionBatchResult;
} & React.ComponentProps<"li">) {
  return (
    <li
      data-testid={`delete-dialog-row-${session.id}`}
      className={cn(
        "grid grid-cols-[12px_minmax(0,1fr)_auto_14px] items-center gap-x-2.5 px-3 py-1.5 text-form",
        className
      )}
      {...props}
    >
      <SessionBadgeMark badge={session.badge} waitingOn={sessionWaitingOn(session)} />
      <span className="truncate">{getSessionDisplayTitle(session)}</span>
      <span
        className={`text-micro ${sessionBadgeWordClass(session.badge, sessionWaitingOn(session))}`}
      >
        {sessionBadgeSignal(session.badge, sessionWaitingOn(session)).displayLabel}
      </span>
      <span className="grid size-3.5 place-items-center">
        {result?.status === "done" ? (
          <StateGlyph label="Deleted" size="sm" state="done" />
        ) : result?.status === "running" ? (
          <StateGlyph label="Deleting" size="sm" state="running" />
        ) : result?.status === "failed" ? (
          <StateGlyph label="Failed" size="sm" state="failed" />
        ) : null}
      </span>
      {result?.status === "failed" ? (
        <span className="col-start-2 col-end-5 text-micro wrap-anywhere text-destructive">
          Couldn't delete: {result.error}
        </span>
      ) : null}
    </li>
  );
}

/** Prioritize in-flight guidance; otherwise disclose active targets only before a partial result. */
function sessionDeleteNote(isDeleting: boolean, partial: boolean, active: number): string | null {
  if (isDeleting) return "Don't close the window while this runs.";
  if (partial || active === 0) return null;
  return active === 1 ? "1 of them is active." : `${active} of them are active.`;
}

/** Disclose every failed target beyond the capped preview without expanding the confirmation list. */
function SessionDeleteOverflowErrors({
  sessions,
  resultsById,
}: {
  sessions: readonly SessionPayload[];
  resultsById: ReadonlyMap<string, SessionBatchResult>;
}) {
  return sessions.map(session => {
    const result = resultsById.get(session.id);
    if (result?.status !== "failed") return null;
    return (
      <p key={session.id} role="alert" className="text-small-body wrap-anywhere text-destructive">
        {getSessionDisplayTitle(session)}: Couldn't delete: {result.error}
      </p>
    );
  });
}
