import { Eraser, Trash2 } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Spinner,
} from "@agh/ui";

import type { SessionPayload } from "@/systems/session";

export function SessionDeleteDialog({
  open,
  onOpenChange,
  session,
  isDeleting,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  session: SessionPayload;
  isDeleting: boolean;
  onConfirm: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
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

export function SessionClearDialog({
  open,
  onOpenChange,
  isClearing,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  isClearing: boolean;
  onConfirm: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={!isClearing}
        className="max-w-md"
        data-testid="composer-clear-dialog"
      >
        <DialogHeader>
          <DialogTitle>Clear conversation</DialogTitle>
          <DialogDescription>
            This removes the visible transcript for this session and starts a fresh runtime
            conversation on the same session id.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter className="gap-2">
          <Button
            type="button"
            variant="ghost"
            onClick={() => onOpenChange(false)}
            disabled={isClearing}
            data-testid="composer-clear-cancel"
          >
            Cancel
          </Button>
          <Button
            type="button"
            variant="destructive"
            onClick={onConfirm}
            disabled={isClearing}
            data-testid="composer-clear-confirm"
          >
            {isClearing ? (
              <>
                <Spinner className="size-3" />
                Clearing
              </>
            ) : (
              <>
                <Eraser className="size-3" />
                Clear conversation
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
