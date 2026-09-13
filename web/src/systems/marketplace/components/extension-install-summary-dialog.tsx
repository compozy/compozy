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

import { ExtensionInstallSummary } from "./extension-install-dialog";
import type { ExtensionInstallPreview, ExtensionInstallRequest } from "@/systems/extensions";

import type { MarketplaceCatalogListing } from "../types";

import { ExtensionInputFields } from "./extension-input-fields";
import { useExtensionInputForm } from "./use-extension-input-form";
import type { ExtensionInputDefinitions, ExtensionInputDraft } from "./extension-install-model";

type SummaryDialogProps = {
  open: boolean;
  pending: boolean;
  onConfirm: (draft: ExtensionInputDraft) => void;
  onOpenChange: (open: boolean) => void;
} & (
  | {
      action: "install";
      preview: ExtensionInstallPreview;
      entry: MarketplaceCatalogListing;
      destination: Pick<ExtensionInstallRequest, "scope" | "profile">;
    }
  | { action: "update"; name: string; definitions: ExtensionInputDefinitions }
);

/**
 * Catalog confirmation step. Install keeps the package summary and asks for every declared input;
 * update recovery asks only for the inputs the daemon reported missing. Both submit the same draft
 * through the owning controller, which retries the already-scoped request.
 */
export function ExtensionInstallSummaryDialog(props: SummaryDialogProps) {
  const { open, pending, onConfirm, onOpenChange } = props;
  const name = props.action === "install" ? props.preview.name : props.name;
  const definitions = props.action === "install" ? props.preview.inputs : props.definitions;
  const form = useExtensionInputForm(definitions, name);
  const actionLabel = props.action === "install" ? "Install" : "Update";
  const hasInputs = definitions.length > 0;
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className="flex max-h-[min(var(--height-modal-md),80vh)] flex-col sm:max-w-(--width-modal-sm)"
        data-testid="extension-install-summary-dialog"
        unframed
      >
        <form
          className="flex min-h-0 flex-1 flex-col"
          onSubmit={event => {
            event.preventDefault();
            if (pending || !form.valid) return;
            onConfirm(form.draft);
          }}
        >
          <DialogHeader variant="ruled">
            <DialogTitle>
              {actionLabel} {name}
            </DialogTitle>
            {props.action === "update" ? (
              <DialogDescription>
                Provide the required inputs to update this extension.
              </DialogDescription>
            ) : (
              <DialogDescription>{props.entry.description}</DialogDescription>
            )}
          </DialogHeader>
          <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-5 py-4">
            {props.action === "install" &&
            (props.preview.declared_profiles.length > 0 || props.preview.placements.length > 0) ? (
              <ExtensionInstallSummary preview={props.preview} />
            ) : null}
            {hasInputs ? <ExtensionInputFields disabled={pending} form={form} /> : null}
            {definitions.some(input => input.type === "secret") ? (
              <p className="text-form-hint text-muted">
                Secrets are stored in your vault. You can change them later from Installed.
              </p>
            ) : null}
            {props.action === "install" ? (
              <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-small-body">
                <dt className="text-muted">Scope</dt>
                <dd data-testid="extension-install-destination">
                  {props.destination.scope === "workspace"
                    ? "Current workspace"
                    : "Everywhere (global)"}
                  {" · "}
                  {props.destination.profile}
                </dd>
              </dl>
            ) : null}
          </div>
          <DialogFooter variant="ruled">
            {props.action === "install" ? (
              <span className="mr-auto text-form-hint text-muted">
                {props.entry.tier === "official" ? "Official" : "Community"}
                {props.entry.trust?.checksum_verified ? " · checksum verified" : ""}
              </span>
            ) : null}
            <Button
              disabled={pending}
              onClick={() => onOpenChange(false)}
              type="button"
              variant="ghost"
            >
              Cancel
            </Button>
            <Button
              data-testid="extension-install-summary-confirm"
              disabled={pending || !form.valid}
              type="submit"
            >
              {pending ? <Spinner aria-hidden="true" className="size-3" /> : null}
              {pending ? (props.action === "install" ? "Installing…" : "Updating…") : actionLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
