import { Trash2 } from "lucide-react";

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

import type { SessionPayload } from "../types";

export interface SessionDeleteDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  session?: SessionPayload | null;
  sessions?: readonly SessionPayload[];
  results?: readonly SessionBatchResult[];
  onRetry?: () => void;
  isDeleting: boolean;
  onConfirm: () => void;
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
          </DialogDescription>
        </DialogHeader>
        <DialogFooter className="gap-2">
          <Button
            type="button"
            variant="ghost"
            onClick={() => onOpenChange(false)}
            disabled={isDeleting}
            data-testid="delete-dialog-cancel"
          >
            Cancel
          </Button>
          <Button
            type="button"
            variant="destructive"
            onClick={onConfirm}
            disabled={isDeleting}
            data-testid="delete-dialog-confirm"
          >
            {isDeleting ? (
              <>
                <Spinner className="size-3" />
                Deleting
              </>
            ) : (
              <>
                <Trash2 className="size-3" />
                Delete session
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
