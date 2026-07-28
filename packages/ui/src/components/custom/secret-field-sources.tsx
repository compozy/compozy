"use client";

import { Plus } from "lucide-react";
import * as React from "react";

import { DIALOG_TOUCH_TARGET_CLASS } from "../../lib/dialog-shell";
import { cn } from "../../lib/utils";
import { Button } from "../button";
import { FieldError } from "../field";
import { Input } from "../input";
import { Spinner } from "../spinner";
import { Eyebrow } from "./eyebrow";
import { Pill } from "./pill";
import { RadioCard } from "./radio-card";

/** An existing stored secret reference this field can bind instead of a typed value. */
export interface SecretFieldSource {
  ref: string;
  /** Whether the store still holds a value for this reference. */
  present: boolean;
}

/** Inline creation of a new stored reference, without leaving the editor. */
export interface SecretFieldSourceCreate {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Reference the new secret will be stored under. */
  ref: string;
  value: string;
  onValueChange: (next: string) => void;
  onSubmit: () => void;
  pending?: boolean;
  error?: React.ReactNode;
  /** Runtime-truthful action label, e.g. `Create Vault secret`. */
  openLabel?: React.ReactNode;
  /** Accessible name for the inline create input. */
  valueLabel?: string;
}

/** Binding a field to the secret store rather than to a typed plaintext value. */
export interface SecretFieldBinding {
  sources: ReadonlyArray<SecretFieldSource>;
  selectedRef: string;
  onSelectRef: (ref: string) => void;
  loading?: boolean;
  error?: React.ReactNode;
  /** Copy shown when the store holds no candidate references. */
  emptyLabel?: React.ReactNode;
  /** Badge describing the reference namespace being listed. */
  namespaceLabel?: React.ReactNode;
  create?: SecretFieldSourceCreate;
}

interface SecretFieldSourcesProps {
  binding: SecretFieldBinding;
  fieldLabel: string;
  testId?: string;
  createTestId?: string;
}

/**
 * Store-reference picker for `SecretField`. Values are never read back — the
 * list carries presence only, and a missing reference cannot be selected.
 */
function SecretFieldSources({
  binding,
  fieldLabel,
  testId,
  createTestId,
}: SecretFieldSourcesProps) {
  const { create } = binding;
  const selectableSources = binding.sources.filter(source => source.present);
  const rovingRef = selectableSources.some(source => source.ref === binding.selectedRef)
    ? binding.selectedRef
    : selectableSources[0]?.ref;

  const handleSourceKeyDown = (
    event: React.KeyboardEvent<HTMLButtonElement>,
    sourceRef: string
  ) => {
    const currentIndex = selectableSources.findIndex(source => source.ref === sourceRef);
    if (currentIndex === -1) return;

    let nextIndex: number;
    switch (event.key) {
      case "ArrowDown":
      case "ArrowRight":
        nextIndex = (currentIndex + 1) % selectableSources.length;
        break;
      case "ArrowUp":
      case "ArrowLeft":
        nextIndex = (currentIndex - 1 + selectableSources.length) % selectableSources.length;
        break;
      case "Home":
        nextIndex = 0;
        break;
      case "End":
        nextIndex = selectableSources.length - 1;
        break;
      default:
        return;
    }

    const nextSource = selectableSources[nextIndex];
    if (!nextSource) return;

    event.preventDefault();
    binding.onSelectRef(nextSource.ref);
    const group = event.currentTarget.closest('[role="radiogroup"]');
    const radios = group?.querySelectorAll<HTMLButtonElement>('[role="radio"]:not(:disabled)');
    radios?.[nextIndex]?.focus();
  };

  return (
    <div
      className="flex flex-col gap-2.5 rounded-md bg-canvas px-3 py-3"
      data-slot="secret-field-sources"
      data-testid={testId}
    >
      {binding.loading ? (
        <div className="flex items-center gap-2 text-small-body text-muted" role="status">
          <Spinner aria-hidden="true" className="size-3" />
          Loading stored references
        </div>
      ) : null}
      {binding.error ? <FieldError>{binding.error}</FieldError> : null}
      {create?.open ? (
        <div className="flex flex-col gap-2">
          <Eyebrow className="text-muted">Create inline</Eyebrow>
          <code className="break-all font-mono text-form-hint text-muted">{create.ref}</code>
          <Input
            aria-label={create.valueLabel ?? `New stored value for ${fieldLabel}`}
            autoComplete="new-password"
            onChange={event => create.onValueChange(event.target.value)}
            placeholder="Secret value"
            type="password"
            value={create.value}
          />
          {create.error ? (
            <p className="text-small-body text-danger" role="alert">
              {create.error}
            </p>
          ) : null}
          <div className="flex items-center gap-2">
            <Button
              data-testid={createTestId}
              className={DIALOG_TOUCH_TARGET_CLASS}
              disabled={create.pending || create.value.trim() === ""}
              onClick={create.onSubmit}
              size="sm"
              type="button"
              variant="neutral"
            >
              {create.pending ? <Spinner aria-hidden="true" className="size-3" /> : null}
              {create.pending ? "Creating…" : "Create secret"}
            </Button>
            <Button
              disabled={create.pending}
              onClick={() => create.onOpenChange(false)}
              size="sm"
              type="button"
              variant="ghost"
            >
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <>
          <div className="flex items-center gap-2">
            <Eyebrow className="text-muted">Choose existing secret</Eyebrow>
            {binding.namespaceLabel ? (
              <Pill className="ml-auto" mono size="xs">
                {binding.namespaceLabel}
              </Pill>
            ) : null}
          </div>
          {!binding.loading && !binding.error && binding.sources.length === 0 ? (
            <p className="text-small-body text-muted">
              {binding.emptyLabel ?? "No stored secrets yet."}
            </p>
          ) : null}
          {binding.sources.length > 0 ? (
            <div
              aria-label={`${fieldLabel} stored references`}
              className="grid gap-1.5"
              role="radiogroup"
            >
              {binding.sources.map(source => (
                <RadioCard
                  badge={
                    <Pill mono size="xs" tone={source.present ? "success" : "danger"}>
                      {source.present ? "present" : "missing"}
                    </Pill>
                  }
                  className={cn(
                    "gap-0 border border-line bg-input-fill px-2.5 py-2 data-[selected]:border-line-strong data-[selected]:bg-row-selected",
                    DIALOG_TOUCH_TARGET_CLASS
                  )}
                  disabled={!source.present}
                  key={source.ref}
                  onKeyDown={event => handleSourceKeyDown(event, source.ref)}
                  onSelect={() => binding.onSelectRef(source.ref)}
                  selected={binding.selectedRef === source.ref}
                  tabIndex={source.ref === rovingRef ? 0 : -1}
                  title={
                    <code className="block break-all font-mono text-mono-id font-normal leading-snug tracking-normal text-fg">
                      {source.ref}
                    </code>
                  }
                  titleClassName="overflow-visible whitespace-normal font-normal tracking-normal"
                />
              ))}
            </div>
          ) : null}
          <div className="flex items-center gap-2">
            <p className="mr-auto text-form-hint text-muted">Values stay hidden.</p>
            {create ? (
              <Button
                onClick={() => create.onOpenChange(true)}
                size="sm"
                type="button"
                variant="outline"
              >
                <Plus aria-hidden="true" className="size-3" />
                {create.openLabel ?? "Create secret"}
              </Button>
            ) : null}
          </div>
        </>
      )}
    </div>
  );
}

export { SecretFieldSources };
