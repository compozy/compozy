import { RotateCcw, Trash2 } from "lucide-react";

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

import { SessionDeleteDialogSet } from "./session-delete-dialog-set";
import type { SessionBatchResult } from "../lib/session-batch";

import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import { sessionProcessChoiceCopy } from "../lib/session-process-choice";
import { sessionRunningProcesses } from "../lib/session-processes";
import type { SessionPayload } from "../types";

export interface SessionDeleteDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  session?: SessionPayload | null;
  sessions?: readonly SessionPayload[];
  results?: readonly SessionBatchResult[];
  onRetry?: () => void;
  isDeleting: boolean;
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
        onConfirm={onConfirm}
      />
    );
  }
  const session = sessions?.[0] ?? singleSession;
  if (!session) return null;
  const failure = results?.find(result => result.id === session.id && result.status === "failed");
  const retrying = failure !== undefined && !isDeleting;
  const processCount = sessionRunningProcesses(session);
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
        {retrying ? (
          <p role="alert" className="text-small-body text-danger">
            Couldn't delete: {failure.error}
          </p>
        ) : null}
        <DialogFooter className="flex-wrap gap-2">
          <Button
            type="button"
            variant="ghost"
            onClick={() => onOpenChange(false)}
            disabled={isDeleting}
            data-testid="delete-dialog-cancel"
          >
            {retrying ? "Close" : "Cancel"}
          </Button>
          {processChoice && !retrying ? (
            <Button
              type="button"
              variant="outline"
              onClick={() => onConfirm("keep")}
              disabled={isDeleting}
              data-testid="delete-dialog-keep-processes"
            >
              {processChoice.keepLabel}
            </Button>
          ) : null}
          <Button
            type="button"
            variant="destructive"
            onClick={retrying ? onRetry : () => onConfirm(processChoice ? "terminate" : undefined)}
            disabled={isDeleting}
            data-testid={retrying ? "delete-dialog-retry" : "delete-dialog-confirm"}
          >
            {isDeleting ? (
              <>
                <Spinner className="size-3" />
                Deleting
              </>
            ) : retrying ? (
              <>
                <RotateCcw className="size-3" />
                Retry 1
              </>
            ) : (
              <>
                <Trash2 className="size-3" />
                {processChoice ? processChoice.confirmLabel : "Delete session"}
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
