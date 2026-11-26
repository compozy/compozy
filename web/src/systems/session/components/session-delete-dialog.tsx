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
  /** Child counts are unresolved, independently of whether a checkbox is visible. */
  childrenPending?: boolean;
  /** The prepared target process count is stable for this dialog lifetime. */
  processCount?: number;
  /** The session has side chats (closed ones included): deleting it deletes them too (S5). */
  sideChats?: boolean;
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
  childrenPending = false,
  processCount,
  sideChats = false,
  onConfirm,
}: SessionDeleteDialogProps) {
  const treeReadPending = childrenPending || childSessions?.pending === true;
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
        childrenPending={treeReadPending}
        processCount={processCount}
        onConfirm={onConfirm}
      />
    );
  }
  const session = sessions?.[0] ?? singleSession;
  if (!session) return null;
  return (
    <SingleSessionDeleteDialog
      open={open}
      onOpenChange={onOpenChange}
      session={session}
      results={results}
      onRetry={onRetry}
      isDeleting={isDeleting}
      childSessions={childSessions}
      childrenPending={treeReadPending}
      processCount={processCount}
      sideChats={sideChats}
      onConfirm={onConfirm}
    />
  );
}

type SingleSessionDeleteDialogProps = Omit<SessionDeleteDialogProps, "session"> & {
  session: SessionPayload;
};

function SingleSessionDeleteDialog({
  open,
  onOpenChange,
  session,
  results,
  onRetry,
  isDeleting,
  childSessions,
  childrenPending = false,
  processCount,
  sideChats = false,
  onConfirm,
}: SingleSessionDeleteDialogProps) {
  const failure = results?.find(result => result.id === session.id && result.status === "failed");
  const retrying = failure !== undefined && !isDeleting;
  const targetProcessCount = processCount ?? sessionRunningProcesses(session);
  const runningProcessCount = targetProcessCount + (childSessions?.includedProcessCount ?? 0);
  const processChoice =
    runningProcessCount > 0 ? sessionProcessChoiceCopy("delete", 1, runningProcessCount) : null;
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
            <SessionDeleteDescription
              session={session}
              processDescription={processChoice?.description}
              sideChats={sideChats}
            />
          </DialogDescription>
        </DialogHeader>
        <SessionDeleteDetails
          childSessions={childSessions}
          childrenPending={childrenPending}
          isDeleting={isDeleting}
          retrying={retrying}
          failureError={failure?.error}
          processChoice={processChoice}
          onOpenChange={onOpenChange}
          onConfirm={onConfirm}
          onRetry={onRetry}
        />
      </DialogContent>
    </Dialog>
  );
}

function SessionDeleteDescription({
  session,
  processDescription,
  sideChats,
}: {
  session: SessionPayload;
  processDescription?: string;
  sideChats: boolean;
}) {
  return (
    <>
      This permanently removes <strong>{session.name?.trim() || session.id}</strong>, including its
      transcript and history, and removes it from the session list.
      {processDescription ? ` ${processDescription}` : null}
      {sideChats ? ` ${SIDE_CHAT_COPY.deleteLine}` : null}
    </>
  );
}

function SessionDeleteDetails({
  childSessions,
  childrenPending,
  isDeleting,
  retrying,
  failureError,
  processChoice,
  onOpenChange,
  onConfirm,
  onRetry,
}: {
  childSessions?: SessionChildrenChoice;
  childrenPending: boolean;
  isDeleting: boolean;
  retrying: boolean;
  failureError?: string;
  processChoice: ReturnType<typeof sessionProcessChoiceCopy> | null;
  onOpenChange: (open: boolean) => void;
  onConfirm: (processes?: SessionProcessPolicy) => void;
  onRetry?: () => void;
}) {
  return (
    <>
      {childSessions ? (
        <SessionChildrenCheckbox
          action="delete"
          choice={childSessions}
          disabled={isDeleting || childrenPending}
          showCheckbox={childSessions.count > 0}
        />
      ) : null}
      {retrying ? (
        <p role="alert" className="text-small-body text-destructive">
          Couldn't delete: {failureError}
        </p>
      ) : null}
      <DialogFooter className="flex-wrap gap-2">
        <SessionDeleteActions
          childrenPending={childrenPending}
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
    </>
  );
}
