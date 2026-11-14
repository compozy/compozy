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
import { Trash2 } from "@compozy/ui/icons";

import type { SubagentDeleteDialogModel } from "../../hooks/use-session-subagent-actions";

export type SubagentDeleteDialogProps = SubagentDeleteDialogModel;

/**
 * Confirmation for permanently removing a delegated subagent and its child
 * session. While the request runs, Escape and outside clicks cannot dismiss the
 * dialog: the outcome (the row leaving the rail, or why it stayed) must be seen.
 */
export function SubagentDeleteDialog({
  open,
  subagent,
  isDeleting,
  failure,
  onOpenChange,
  onConfirm,
}: SubagentDeleteDialogProps) {
  if (!subagent) return null;
  return (
    <Dialog
      open={open}
      disablePointerDismissal={isDeleting}
      onOpenChange={(nextOpen, details) => {
        if (!nextOpen && isDeleting) {
          details.cancel();
          return;
        }
        onOpenChange(nextOpen);
      }}
    >
      <DialogContent
        showCloseButton={!isDeleting}
        className="max-w-md"
        data-testid="subagent-delete-dialog"
      >
        <DialogHeader>
          <DialogTitle>Delete subagent</DialogTitle>
          <DialogDescription>
            This permanently removes <strong>{subagent.title}</strong> and its session transcript
            and history. The subagent cannot be recovered.
          </DialogDescription>
        </DialogHeader>
        {failure ? (
          <p role="alert" className="text-small-body text-destructive">
            Couldn't delete: {failure}
          </p>
        ) : null}
        <DialogFooter className="flex-wrap gap-2">
          <Button
            type="button"
            variant="ghost"
            disabled={isDeleting}
            onClick={() => onOpenChange(false)}
            data-testid="subagent-delete-dialog-cancel"
          >
            Cancel
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={isDeleting}
            onClick={onConfirm}
            data-testid="subagent-delete-dialog-confirm"
          >
            {isDeleting ? (
              <>
                <Spinner className="size-3 motion-reduce:animate-none" />
                Deleting
              </>
            ) : (
              <>
                <Trash2 className="size-3" />
                Delete subagent
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
