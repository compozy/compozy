import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@compozy/ui";

import { SIDE_CHAT_COPY } from "../../lib/session-side";

/** "Close side chat" while it is answering asks first (US-008.AC-2). */
export function SessionSideCloseDialog({
  open,
  closing,
  onOpenChange,
  onConfirm,
}: {
  open: boolean;
  closing: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={next => onOpenChange(next)}>
      <DialogContent className="max-w-sm" data-testid="side-chat-close-dialog">
        <DialogHeader>
          <DialogTitle>{SIDE_CHAT_COPY.closeConfirmTitle}</DialogTitle>
          <DialogDescription>{SIDE_CHAT_COPY.closeConfirmBody}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button onClick={() => onOpenChange(false)} type="button" variant="ghost">
            Cancel
          </Button>
          <Button
            data-testid="side-chat-close-confirm"
            disabled={closing}
            onClick={onConfirm}
            type="button"
            variant="destructive"
          >
            {SIDE_CHAT_COPY.closeConfirm}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
