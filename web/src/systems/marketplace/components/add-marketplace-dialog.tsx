import { AlertCircle, Check } from "lucide-react";

import {
  ActionResultBanner,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  FieldDescription,
  FieldHeader,
  FieldLabel,
  Input,
  MonoId,
  Spinner,
} from "@compozy/ui";

import type { MarketplaceSource } from "../types";
import { previewSummary, type AddMarketplaceFailure } from "./add-marketplace-model";
import { useAddMarketplaceForm } from "./use-add-marketplace-form";

const REF_ID = "add-marketplace-ref";
const NAME_ID = "add-marketplace-name";

export interface AddMarketplaceDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Fires after the daemon registered the source; the dialog closes itself first. */
  onAdded?: (source: MarketplaceSource) => void;
}

/**
 * One field, a check, then Add. The notice restates what the daemon found (name, owner, plugin
 * counts, document path) or exactly why it refused (both checked paths, the suggested name, the
 * extensions retaining a name). The primary is the dialog's one accent and enables only after a
 * successful check of the current reference and name.
 */
export function AddMarketplaceDialog({ open, onOpenChange, onAdded }: AddMarketplaceDialogProps) {
  const form = useAddMarketplaceForm({ onOpenChange, onAdded });

  return (
    <Dialog onOpenChange={form.close} open={open}>
      <DialogContent
        className="flex max-h-[min(var(--height-modal-md),80vh)] flex-col sm:max-w-(--width-modal-sm)"
        data-testid="add-marketplace-dialog"
        unframed
      >
        <form
          className="flex min-h-0 flex-1 flex-col"
          onSubmit={event => {
            event.preventDefault();
            form.submit();
          }}
        >
          <DialogHeader variant="ruled">
            <DialogTitle>Add a plugin marketplace</DialogTitle>
            <DialogDescription>
              CompozyOS reads its plugin list and shows the plugins in the Marketplace. Nothing
              installs until you choose to.
            </DialogDescription>
          </DialogHeader>

          <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-5 py-4">
            <Field data-invalid={form.refInvalid ? true : undefined}>
              <FieldHeader>
                <FieldLabel htmlFor={REF_ID}>GitHub repository or folder</FieldLabel>
              </FieldHeader>
              <Input
                aria-describedby={`${REF_ID}-hint`}
                aria-invalid={form.refInvalid ? true : undefined}
                autoComplete="off"
                autoFocus
                className="font-mono"
                data-testid={REF_ID}
                disabled={form.isPending}
                id={REF_ID}
                onBlur={form.check}
                onChange={form.editRef}
                placeholder="owner/repo · https://github.com/owner/repo · /path/to/folder"
                spellCheck={false}
                value={form.draft.ref}
              />
              <FieldDescription id={`${REF_ID}-hint`}>
                The repository needs a <MonoId value="marketplace.json" /> at its root or under{" "}
                <MonoId value=".claude-plugin/" />.
              </FieldDescription>
            </Field>

            {form.showNameField ? (
              <Field data-invalid={form.nameInvalid ? true : undefined}>
                <FieldHeader>
                  <FieldLabel htmlFor={NAME_ID}>Name</FieldLabel>
                </FieldHeader>
                <Input
                  aria-describedby={`${NAME_ID}-hint`}
                  aria-invalid={form.nameInvalid ? true : undefined}
                  autoComplete="off"
                  className="font-mono"
                  data-testid={NAME_ID}
                  disabled={form.isPending}
                  id={NAME_ID}
                  onBlur={form.check}
                  onChange={form.editName}
                  placeholder="team-plugins"
                  spellCheck={false}
                  value={form.draft.name}
                />
                <FieldDescription id={`${NAME_ID}-hint`}>
                  Optional. Leave empty to use the repository or folder name.
                </FieldDescription>
              </Field>
            ) : null}

            {form.checking ? (
              <p
                className="flex items-center gap-2 text-small-body text-subtle"
                data-testid="add-marketplace-checking"
                role="status"
              >
                <Spinner aria-hidden="true" className="size-3" />
                Reading the plugin list…
              </p>
            ) : null}

            {form.found ? (
              <ActionResultBanner
                data-testid="add-marketplace-found"
                description={
                  <>
                    Read from <MonoId value={form.found.document_path} />.
                    {form.found.plugins === 0
                      ? " Nothing will show in the Marketplace until this list has plugins."
                      : null}
                  </>
                }
                icon={Check}
                title={previewSummary(form.found)}
                tone="success"
              />
            ) : null}

            {form.failure ? (
              <AddMarketplaceFailureNotice
                disabled={form.isPending}
                failure={form.failure}
                onUseSuggestedName={form.useSuggestedName}
              />
            ) : null}
          </div>

          <DialogFooter variant="ruled">
            {form.found ? (
              <span className="mr-auto text-form-hint text-subtle">
                Refreshes with the catalog.
              </span>
            ) : null}
            <Button
              data-testid="add-marketplace-cancel"
              disabled={form.isPending}
              onClick={form.cancel}
              type="button"
              variant="ghost"
            >
              Cancel
            </Button>
            <Button data-testid="add-marketplace-submit" disabled={!form.canAdd} type="submit">
              {form.isPending ? <Spinner aria-hidden="true" className="size-3" /> : null}
              {form.isPending ? "Adding…" : "Add marketplace"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

/** Plain sentence first, the daemon's code after in micro mono; one action when it proposed one. */
function AddMarketplaceFailureNotice({
  failure,
  disabled,
  onUseSuggestedName,
}: {
  failure: AddMarketplaceFailure;
  disabled: boolean;
  onUseSuggestedName: (name: string) => void;
}) {
  const suggested = failure.suggestedName;
  return (
    <ActionResultBanner
      actions={
        suggested ? (
          <Button
            data-testid="add-marketplace-use-suggested"
            disabled={disabled}
            onClick={() => onUseSuggestedName(suggested)}
            size="xs"
            type="button"
            variant="outline"
          >
            Use {suggested}
          </Button>
        ) : undefined
      }
      data-code={failure.code ?? undefined}
      data-testid="add-marketplace-failure"
      description={
        <>
          {failure.description}
          {failure.code ? (
            <>
              {" "}
              <MonoId className="text-faint" preserveCase value={failure.code} />
            </>
          ) : null}
        </>
      }
      icon={AlertCircle}
      role="alert"
      title={failure.title}
      tone={failure.tone}
    />
  );
}
