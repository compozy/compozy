import { Check, Copy, KeyRound, X } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { Button, Eyebrow, Input, Pill, Sheet, SheetContent, Time } from "@agh/ui";

import { vaultNamespaceTone } from "../lib/vault-tones";
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
  const [copied, setCopied] = useState(false);
  const [copyError, setCopyError] = useState(false);
  const copyResetTimerRef = useRef<number | null>(null);

  useEffect(
    () => () => {
      if (copyResetTimerRef.current) window.clearTimeout(copyResetTimerRef.current);
    },
    []
  );

  const handleCopy = async () => {
    if (!secret || !navigator.clipboard) {
      setCopied(false);
      setCopyError(true);
      return;
    }
    try {
      await navigator.clipboard.writeText(secret.ref);
      setCopyError(false);
      setCopied(true);
      if (copyResetTimerRef.current) window.clearTimeout(copyResetTimerRef.current);
      copyResetTimerRef.current = window.setTimeout(() => {
        setCopied(false);
        copyResetTimerRef.current = null;
      }, 1500);
    } catch {
      setCopied(false);
      setCopyError(true);
    }
  };

  return (
    <Sheet
      open={open}
      onOpenChange={next => {
        if (!next) {
          setCopied(false);
          setCopyError(false);
        }
        onOpenChange(next);
      }}
    >
      <SheetContent
        className="grid w-full grid-rows-[auto_1fr_auto] gap-0 p-0 sm:max-w-[32.5rem]"
        data-testid="vault-secret-sheet"
        showCloseButton={false}
        side="right"
      >
        {secret ? (
          <>
            <SheetHead
              copied={copied}
              copyError={copyError}
              onClose={() => onOpenChange(false)}
              onCopy={handleCopy}
              secret={secret}
            />
            <div className="min-h-0 overflow-y-auto px-5 py-4.5">
              <SheetTiles secret={secret} />
              <SheetValueSection present={secret.present} />
              <SheetReplaceSection
                error={replaceError}
                isPending={replaceIsPending}
                isValid={replaceIsValid}
                onReplace={onReplace}
                onReplaceValueChange={onReplaceValueChange}
                replaceValue={replaceValue}
              />
              <SheetDangerZone
                disabled={deleteIsDisabled}
                onRequestDelete={() => onRequestDelete(secret)}
              />
            </div>
            <footer
              className="flex items-center gap-2.5 border-t border-line-soft px-5 py-3"
              data-testid="vault-secret-sheet-foot"
            >
              <span className="min-w-0 truncate font-mono text-mono-id text-faint">
                agh vault put {secret.ref} --value-stdin
              </span>
            </footer>
          </>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}

function SheetHead({
  secret,
  copied,
  copyError,
  onCopy,
  onClose,
}: {
  secret: VaultSecret;
  copied: boolean;
  copyError: boolean;
  onCopy: () => void;
  onClose: () => void;
}) {
  return (
    <header className="flex items-start gap-3 border-b border-line px-5 py-4.5">
      <span
        aria-hidden="true"
        className="grid size-9 shrink-0 place-items-center rounded-md bg-accent-tint text-accent-strong"
      >
        <KeyRound className="size-4" />
      </span>
      <div className="min-w-0 flex-1">
        <Eyebrow className="text-subtle">Vault secret</Eyebrow>
        <h2
          className="mt-0.5 break-all text-item-title font-medium tracking-tight text-fg-strong"
          data-testid="vault-secret-sheet-title"
          id="vault-secret-sheet-title"
        >
          {vaultSecretTitle(secret.ref)}
        </h2>
        <div className="mt-1.5 flex min-w-0 items-center gap-1.5 font-mono text-mono-id text-muted">
          <span className="min-w-0 truncate" data-testid="vault-secret-sheet-ref">
            {secret.ref}
          </span>
          <button
            aria-label={copied ? "Copied vault reference" : "Copy vault reference"}
            className="inline-grid size-6 shrink-0 place-items-center rounded-xxs text-faint transition-colors hover:bg-row-hover hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring"
            data-testid="vault-secret-sheet-copy"
            onClick={onCopy}
            type="button"
          >
            {copied ? (
              <Check aria-hidden="true" className="size-[11px] text-success" />
            ) : (
              <Copy aria-hidden="true" className="size-[11px]" />
            )}
          </button>
        </div>
        {copyError ? (
          <p
            className="mt-1 text-caption text-danger"
            data-testid="vault-secret-sheet-copy-error"
            role="alert"
          >
            Vault reference could not be copied.
          </p>
        ) : null}
      </div>
      <button
        aria-label="Close"
        className="inline-grid size-[26px] shrink-0 place-items-center rounded-sm text-muted transition-colors hover:bg-row-hover hover:text-fg-strong"
        data-testid="vault-secret-sheet-close"
        onClick={onClose}
        type="button"
      >
        <X aria-hidden="true" className="size-3.5" />
      </button>
    </header>
  );
}

function SheetTiles({ secret }: { secret: VaultSecret }) {
  const trimmedKind = secret.kind?.trim();
  return (
    <div className="mb-4 grid grid-cols-2 gap-2.5" data-testid="vault-secret-sheet-tiles">
      <Tile label="Namespace">
        <Pill mono size="sm" tone={vaultNamespaceTone(secret.namespace)}>
          {secret.namespace}
        </Pill>
      </Tile>
      <Tile label="Kind">
        <span className="font-mono text-mono-id text-muted">{trimmedKind || "--"}</span>
      </Tile>
      <Tile label="Created">
        <Time className="text-small-body font-medium text-fg" iso={secret.created_at} />
      </Tile>
      <Tile label="Updated">
        <Time className="text-small-body font-medium text-fg" iso={secret.updated_at} />
      </Tile>
    </div>
  );
}

function Tile({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="rounded-md border border-line-soft bg-input-fill px-3.5 py-2.5">
      <Eyebrow className="mb-1.5 text-subtle">{label}</Eyebrow>
      <div className="flex min-w-0 items-center gap-1.5">{children}</div>
    </div>
  );
}

function SheetValueSection({ present }: { present: boolean }) {
  return (
    <section className="mb-4.5" data-testid="vault-secret-sheet-value">
      <Eyebrow className="mb-2.5 text-subtle">Value</Eyebrow>
      <div className="flex items-center gap-2.5 rounded-md border border-line-soft bg-input-fill px-3.5 py-2.5">
        <span aria-hidden="true" className="flex-1 font-mono text-form-input text-faint">
          • • • • • • • •
        </span>
        {present ? (
          <Pill size="sm" tone="success">
            present
          </Pill>
        ) : (
          <Pill size="sm" tone="warning">
            missing
          </Pill>
        )}
        <Pill mono size="sm" tone="neutral">
          write-only
        </Pill>
      </div>
      <p className="mt-1.5 text-form-hint leading-normal text-subtle">
        The daemon never returns secret values. Reads expose redacted metadata only.
      </p>
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
          placeholder="Stored without plaintext readback"
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
          variant="outline"
        >
          {isPending ? "Storing…" : "Store"}
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
          Re-storing overwrites the value in place — the rotate path. Kind metadata is preserved.
        </p>
      )}
    </section>
  );
}

function SheetDangerZone({
  disabled,
  onRequestDelete,
}: {
  disabled: boolean;
  onRequestDelete: () => void;
}) {
  return (
    <section data-testid="vault-secret-sheet-danger">
      <div className="flex items-center justify-between gap-3 rounded-md border border-danger/20 bg-danger-tint px-3.5 py-3">
        <div className="min-w-0">
          <b className="block text-small-body font-medium text-danger">Delete secret</b>
          <p className="mt-0.5 text-form-hint leading-snug text-muted">
            Configs that reference this ref will fail to resolve it at spawn time.
          </p>
        </div>
        <Button
          data-testid="vault-secret-sheet-delete"
          disabled={disabled}
          onClick={onRequestDelete}
          size="sm"
          type="button"
          variant="destructive"
        >
          Delete
        </Button>
      </div>
    </section>
  );
}
