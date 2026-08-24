import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Spinner,
} from "@compozy/ui";

import { ExtensionInstallSummary } from "./extension-install-dialog";
import type { ExtensionInstallPreview } from "@/systems/extensions";

export function ExtensionInstallSummaryDialog({
  open,
  pending,
  preview,
  onConfirm,
  onOpenChange,
}: {
  open: boolean;
  pending: boolean;
  preview: ExtensionInstallPreview;
  onConfirm: () => void;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent className="sm:max-w-(--width-modal-sm)" unframed>
        <DialogHeader variant="ruled">
          <DialogTitle>Install {preview.name}</DialogTitle>
        </DialogHeader>
        <div className="px-5 py-4">
          <ExtensionInstallSummary preview={preview} />
        </div>
        <DialogFooter variant="ruled">
          <Button
            disabled={pending}
            onClick={() => onOpenChange(false)}
            type="button"
            variant="ghost"
          >
            Cancel
          </Button>
          <Button disabled={pending} onClick={onConfirm} type="button">
            {pending ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {pending ? "Installing…" : "Install"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
