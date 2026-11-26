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
import { SIDE_CHAT_COPY } from "../lib/session-side";
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
  /** The session has side chats (closed ones included): deleting it deletes them too (S5). */
  sideChats?: boolean;
  /** Metadata is still loading, so reserve space for side-chat and process choices. */
  metadataPending?: boolean;
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
  sideChats = false,
  metadataPending = false,
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
        metadataPending={metadataPending}
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
      disablePointerDismissal={isDeleting ? true : undefined}
      onOpenChange={(nextOpen, details) => {
        // Hold the confirmation through its request: cancel Escape/outside dismissals instead
        // of letting the dialog replay an open/close transition under the operator.
        if (isDeleting && !nextOpen) {
          details.cancel();
          return;
        }
        onOpenChange(nextOpen);
      }}
    >
      <DialogContent showCloseButton={!isDeleting} className="max-w-md" data-testid="delete-dialog">
        <DialogHeader>
          <DialogTitle>Delete session</DialogTitle>
          <DialogDescription>
            This permanently removes <strong>{session.name?.trim() || session.id}</strong>,
            including its transcript and history, and removes it from the session list.
            <span
              data-testid="delete-dialog-process-description-placeholder"
              className={!processChoice && metadataPending ? "invisible" : undefined}
              aria-hidden={!processChoice && metadataPending}
            >
              {processChoice
                ? ` ${processChoice.description}`
                : metadataPending
                  ? " Reserved for process choice."
                  : null}
            </span>
            <span
              data-testid="delete-dialog-side-chat-placeholder"
              className={!sideChats && metadataPending ? "invisible" : undefined}
              aria-hidden={!sideChats && metadataPending}
            >
              {sideChats
                ? ` ${SIDE_CHAT_COPY.deleteLine}`
                : metadataPending
                  ? " Associated side chats will also be deleted."
                  : null}
            </span>
          </DialogDescription>
        </DialogHeader>
        {childSessions && (childSessions.count > 0 || !childSessions.pending) ? (
          <SessionChildrenCheckbox action="delete" choice={childSessions} disabled={isDeleting} />
        ) : null}
        {retrying ? (
          <p role="alert" className="text-small-body text-destructive">
            Couldn't delete: {failure.error}
          </p>
        ) : null}
        <DialogFooter className="flex-wrap gap-2">
          <SessionDeleteActions
            childrenPending={childSessions?.pending === true}
            askOnRetry={(childSessions?.includedProcessCount ?? 0) > 0}
            isDeleting={isDeleting}
            retrying={retrying}
            processChoice={processChoice}
            processChoicePending={metadataPending && !processChoice}
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
