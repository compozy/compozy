import { Eraser } from "lucide-react";

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
