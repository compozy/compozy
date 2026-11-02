import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@compozy/ui";

import { SessionDeleteActions } from "./session-delete-actions";
import { SessionChildrenCheckbox } from "./session-children-checkbox";
import { SessionDeleteDialogSet } from "./session-delete-dialog-set";
import type { SessionBatchResult } from "../lib/session-batch";

import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import { sessionProcessChoiceCopy } from "../lib/session-process-choice";
import { sessionRunningProcesses } from "../lib/session-processes";
import type { SessionChildrenChoice } from "../lib/session-tree";
import type { SessionPayload } from "../types";

export interface SessionDeleteDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  session?: SessionPayload | null;
  sessions?: readonly SessionPayload[];
  results?: readonly SessionBatchResult[];
  onRetry?: () => void;
  isDeleting: boolean;
  /** Present when the targets have child sessions; absent keeps the dialog unchanged. */
  childSessions?: SessionChildrenChoice;
  /** `processes` answers what happens to running processes; absent when none run. */
  onConfirm: (processes?: SessionProcessPolicy) => void;
}

/** Shared confirmation surface for every session deletion entry point. */
export function SessionDeleteDialog({
  open,
  onOpenChange,
  session: singleSession,
  sessions,
  results,
  onRetry,
  isDeleting,
  childSessions,
  onConfirm,
}: SessionDeleteDialogProps) {
  if (sessions && sessions.length > 1) {
    return (
      <SessionDeleteDialogSet
        open={open}
        onOpenChange={onOpenChange}
        sessions={sessions}
        results={results}
        onRetry={onRetry}
        isDeleting={isDeleting}
        childSessions={childSessions}
        onConfirm={onConfirm}
      />
    );
  }
  const session = sessions?.[0] ?? singleSession;
  if (!session) return null;
  const failure = results?.find(result => result.id === session.id && result.status === "failed");
  const retrying = failure !== undefined && !isDeleting;
  const processCount =
    sessionRunningProcesses(session) + (childSessions?.includedProcessCount ?? 0);
  const processChoice =
    processCount > 0 ? sessionProcessChoiceCopy("delete", 1, processCount) : null;
  return (
    <Dialog
      open={open}
      onOpenChange={nextOpen => {
        if (isDeleting && !nextOpen) return;
        onOpenChange(nextOpen);
      }}
    >
      <DialogContent showCloseButton={!isDeleting} className="max-w-md" data-testid="delete-dialog">
        <DialogHeader>
          <DialogTitle>Delete session</DialogTitle>
          <DialogDescription>
            This permanently removes <strong>{session.name?.trim() || session.id}</strong>,
            including its transcript and history, and removes it from the session list.
            {processChoice ? ` ${processChoice.description}` : null}
          </DialogDescription>
        </DialogHeader>
        {childSessions ? (
          <SessionChildrenCheckbox action="delete" choice={childSessions} disabled={isDeleting} />
        ) : null}
        {retrying ? (
          <p role="alert" className="text-small-body text-destructive">
            Couldn't delete: {failure.error}
          </p>
        ) : null}
        <DialogFooter className="flex-wrap gap-2">
          <SessionDeleteActions
            childrenPending={childSessions?.included === true && childSessions.pending}
            askOnRetry={(childSessions?.includedProcessCount ?? 0) > 0}
            isDeleting={isDeleting}
            retrying={retrying}
            processChoice={processChoice}
            deleteLabel="Delete session"
            deletingLabel="Deleting"
            retryLabel="Retry 1"
            onClose={() => onOpenChange(false)}
            onConfirm={onConfirm}
            onRetry={onRetry}
          />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
