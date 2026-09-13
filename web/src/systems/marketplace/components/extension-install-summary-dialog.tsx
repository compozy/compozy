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

import { useExtensionInputForm } from "./use-extension-input-form";
import type { ExtensionInputDefinitions, ExtensionInputDraft } from "./extension-install-model";

type SummaryDialogProps = {
  open: boolean;
  pending: boolean;
  onConfirm: (draft: ExtensionInputDraft) => void;
  onOpenChange: (open: boolean) => void;
} & (
  | { action: "install"; preview: ExtensionInstallPreview }
  | { action: "update"; name: string; definitions: ExtensionInputDefinitions }
);

export function ExtensionInstallSummaryDialog(props: SummaryDialogProps) {
  const { open, pending, onConfirm, onOpenChange } = props;
  const name = props.action === "install" ? props.preview.name : props.name;
  const definitions = props.action === "install" ? props.preview.inputs : props.definitions;
  const form = useExtensionInputForm(definitions, name);
  const actionLabel = props.action === "install" ? "Install" : "Update";
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent className="sm:max-w-(--width-modal-sm)" unframed>
        <DialogHeader variant="ruled">
          <DialogTitle>
            {actionLabel} {name}
          </DialogTitle>
        </DialogHeader>
        <div className="px-5 py-4">
          {props.action === "install" ? <ExtensionInstallSummary preview={props.preview} /> : null}
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
          <Button
            disabled={pending || !form.valid}
            onClick={() => onConfirm(form.draft)}
            type="button"
          >
            {pending ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {pending ? (props.action === "install" ? "Installing…" : "Updating…") : actionLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
