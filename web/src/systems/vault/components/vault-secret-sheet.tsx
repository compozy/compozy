import { KeyRound, X } from "lucide-react";

import {
  Button,
  CopyIconButton,
  Eyebrow,
  Input,
  MetadataList,
  Pill,
  Sheet,
  SheetContent,
  Time,
} from "@compozy/ui";

import { vaultSecretTitle } from "../lib/vault-secret-title";
import type { VaultSecret } from "../types";

export interface VaultSecretSheetProps {
  secret: VaultSecret | null;
  open: boolean;
  replaceValue: string;
  replaceIsValid: boolean;
  replaceIsPending: boolean;
  deleteIsDisabled: boolean;
  replaceError: string | null;
  onOpenChange: (open: boolean) => void;
  onReplaceValueChange: (value: string) => void;
  onReplace: () => void;
  onRequestDelete: (secret: VaultSecret) => void;
}

export function VaultSecretSheet({
  secret,
  open,
  replaceValue,
  replaceIsValid,
  replaceIsPending,
  deleteIsDisabled,
  replaceError,
  onOpenChange,
  onReplaceValueChange,
  onReplace,
  onRequestDelete,
}: VaultSecretSheetProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        className="grid w-full grid-rows-[auto_1fr_auto] gap-0 p-0 sm:max-w-(--width-modal-sm)"
        data-testid="vault-secret-sheet"
        showCloseButton={false}
        side="right"
      >
        {secret ? (
          <>
            <SheetHead onClose={() => onOpenChange(false)} secret={secret} />
            <div className="min-h-0 overflow-y-auto px-5 py-4.5">
              <SheetFacts secret={secret} />
              <SheetValueSection present={secret.present} />
              <SheetReplaceSection
                error={replaceError}
                isPending={replaceIsPending}
                isValid={replaceIsValid}
                onReplace={onReplace}
                onReplaceValueChange={onReplaceValueChange}
                replaceValue={replaceValue}
              />
            </div>
            <footer
              className="flex items-center justify-between gap-3 border-t border-line-soft px-5 py-3"
              data-testid="vault-secret-sheet-danger"
            >
              <p className="min-w-0 text-form-hint leading-snug text-muted">
                Anything that uses this secret stops working.
              </p>
              <Button
                data-testid="vault-secret-sheet-delete"
                disabled={deleteIsDisabled}
                onClick={() => onRequestDelete(secret)}
                size="sm"
                type="button"
                variant="destructive"
              >
                Delete
              </Button>
            </footer>
          </>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}

function SheetHead({ secret, onClose }: { secret: VaultSecret; onClose: () => void }) {
  return (
    <header className="flex items-start gap-3 border-b border-line px-5 py-4.5">
      <span
        aria-hidden="true"
        className="grid size-9 shrink-0 place-items-center rounded-md bg-surface-2 text-muted"
      >
        <KeyRound className="size-4" />
      </span>
      <div className="min-w-0 flex-1">
        <h2
          className="break-all text-item-title font-medium tracking-tight text-fg"
          data-testid="vault-secret-sheet-title"
          id="vault-secret-sheet-title"
        >
          {vaultSecretTitle(secret.ref)}
        </h2>
        <div className="mt-1 flex min-w-0 items-center gap-1 font-mono text-mono-id text-muted">
          <span className="min-w-0 truncate" data-testid="vault-secret-sheet-ref">
            {secret.ref}
          </span>
          <CopyIconButton
            copiedLabel="Copied name"
            copiedToastLabel="Secret name copied"
            copyFailedLabel="Couldn't copy name"
            copyFailedToastLabel="Couldn't copy the secret name"
            copyLabel="Copy secret name"
            data-testid="vault-secret-sheet-copy"
            value={secret.ref}
          />
        </div>
      </div>
      <Button
        aria-label="Close"
        data-testid="vault-secret-sheet-close"
        onClick={onClose}
        size="icon-sm"
        type="button"
        variant="ghost"
      >
        <X aria-hidden="true" className="size-3.5" />
      </Button>
    </header>
  );
}

function SheetFacts({ secret }: { secret: VaultSecret }) {
  const trimmedKind = secret.kind?.trim();
  const edited = secret.created_at !== secret.updated_at;
  return (
    <MetadataList className="mb-4" data-testid="vault-secret-sheet-facts">
      <MetadataList.Row label="Updated">
        <Time className="text-small-body text-fg" iso={secret.updated_at} />
      </MetadataList.Row>
      {edited ? (
        <MetadataList.Row label="Created">
          <Time className="text-small-body text-fg" iso={secret.created_at} />
        </MetadataList.Row>
      ) : null}
      {trimmedKind ? <MetadataList.Row label="Label">{trimmedKind}</MetadataList.Row> : null}
    </MetadataList>
  );
}

function SheetValueSection({ present }: { present: boolean }) {
  return (
    <section className="mb-4.5" data-testid="vault-secret-sheet-value">
      <Eyebrow className="mb-2.5 text-subtle">Value</Eyebrow>
      <div className="flex items-center gap-2.5 rounded-lg bg-sunken px-3.5 py-2.5">
        <span aria-hidden="true" className="flex-1 font-mono text-form-input text-faint">
          • • • • • • • •
        </span>
        {present ? (
          <span className="text-small-body text-muted">Saved</span>
        ) : (
          <Pill size="sm" tone="warning">
            Missing
          </Pill>
        )}
      </div>
    </section>
  );
}

function SheetReplaceSection({
  replaceValue,
  isValid,
  isPending,
  error,
  onReplaceValueChange,
  onReplace,
}: {
  replaceValue: string;
  isValid: boolean;
  isPending: boolean;
  error: string | null;
  onReplaceValueChange: (value: string) => void;
  onReplace: () => void;
}) {
  return (
    <section className="mb-4.5" data-testid="vault-secret-sheet-replace">
      <Eyebrow className="mb-2.5 text-subtle">Replace value</Eyebrow>
      <div className="flex gap-2">
        <Input
          aria-label="New secret value"
          autoComplete="off"
          className="min-w-0 flex-1 font-mono"
          data-testid="vault-secret-sheet-replace-input"
          onChange={event => onReplaceValueChange(event.target.value)}
          placeholder="Paste the new value"
          spellCheck={false}
          type="password"
          value={replaceValue}
        />
        <Button
          data-testid="vault-secret-sheet-replace-save"
          disabled={!isValid || isPending}
          onClick={onReplace}
          size="sm"
          type="button"
          variant="secondary"
        >
          {isPending ? "Saving…" : "Save"}
        </Button>
      </div>
      {error ? (
        <p
          className="mt-1.5 text-form-hint text-danger"
          data-testid="vault-secret-sheet-replace-error"
        >
          {error}
        </p>
      ) : (
        <p className="mt-1.5 text-form-hint leading-normal text-subtle">
          Replacing updates everything that uses this secret.
        </p>
      )}
    </section>
  );
}
