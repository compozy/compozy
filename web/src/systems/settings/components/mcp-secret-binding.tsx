import {
  Button,
  cn,
  Eyebrow,
  Input,
  NativeSelect,
  NativeSelectOption,
  Pill,
  RadioCard,
  Spinner,
} from "@agh/ui";

import type { MCPSecretBinding } from "../lib/mcp-editor-model";

export type MCPVaultInventory =
  | { status: "loading" }
  | { status: "error"; message: string; retry: () => void }
  | { status: "ready"; refs: string[] };

export interface MCPSecretBindingControlProps {
  binding: MCPSecretBinding;
  /** Explicit, query-backed `vault:mcp/**` inventory state. */
  vaultInventory: MCPVaultInventory;
  idPrefix: string;
  valueLabel?: string;
  onChange: (binding: MCPSecretBinding) => void;
}

/**
 * Presence-safe secret editor: preserve a configured binding without revealing
 * it, write a replacement value, or select a separate Vault inventory entry.
 */
export function MCPSecretBindingControl({
  binding,
  vaultInventory,
  idPrefix,
  valueLabel = "New secret value",
  onChange,
}: MCPSecretBindingControlProps) {
  const refs = vaultInventory.status === "ready" ? vaultInventory.refs : [];
  const vaultAvailable = refs.length > 0;

  return (
    <div>
      <div
        className={cn("grid gap-1.5", binding.existing ? "grid-cols-3" : "grid-cols-2")}
        role="radiogroup"
        aria-label="Secret binding source"
      >
        {binding.existing ? (
          <RadioCard
            selected={binding.mode === "preserve"}
            title="Keep configured"
            description="Retain the current binding"
            badge={
              <Pill size="xs" tone="success">
                Configured
              </Pill>
            }
            onSelect={() => onChange({ ...binding, mode: "preserve" })}
            data-testid={`${idPrefix}-mode-preserve`}
          />
        ) : null}
        <RadioCard
          selected={binding.mode === "typed"}
          title="Write a new value"
          description="Creates its canonical Vault ref"
          onSelect={() => onChange({ ...binding, mode: "typed" })}
          data-testid={`${idPrefix}-mode-typed`}
        />
        <RadioCard
          selected={binding.mode === "ref"}
          title="Use Vault"
          description={vaultAvailable ? "Select an existing credential" : "Inventory unavailable"}
          disabled={!vaultAvailable}
          className="disabled:cursor-not-allowed disabled:opacity-50"
          onSelect={() =>
            onChange({
              ...binding,
              mode: "ref",
              vaultRef: binding.vaultRef || refs[0],
            })
          }
          data-testid={`${idPrefix}-mode-ref`}
        />
      </div>

      {binding.mode === "preserve" ? (
        <p className="mt-2 text-caption text-muted">
          AGH keeps the configured binding without returning or displaying its identifier.
        </p>
      ) : binding.mode === "typed" ? (
        <div className="mt-2">
          <label
            className="mb-1.5 flex items-center gap-2 text-form-label font-medium text-fg"
            htmlFor={`${idPrefix}-typed`}
          >
            {valueLabel}
            <Eyebrow className="ml-auto text-muted">write-only</Eyebrow>
          </label>
          <Input
            id={`${idPrefix}-typed`}
            type="password"
            autoComplete="new-password"
            className="font-mono"
            value={binding.typedValue}
            placeholder="Enter replacement value"
            onChange={event => onChange({ ...binding, typedValue: event.target.value })}
            data-testid={`${idPrefix}-typed-input`}
          />
          <p className="mt-1.5 text-caption text-muted">
            AGH stores this value on save. The value and resulting binding are never returned.
          </p>
        </div>
      ) : (
        <div className="mt-2">
          <label
            className="mb-1.5 flex items-center gap-2 text-form-label font-medium text-fg"
            htmlFor={`${idPrefix}-ref`}
          >
            Existing Vault ref
          </label>
          {vaultInventory.status === "loading" ? (
            <p
              className="flex items-center gap-2 text-caption text-muted"
              data-testid={`${idPrefix}-ref-loading`}
            >
              <Spinner className="size-3" />
              Loading Vault inventory…
            </p>
          ) : vaultInventory.status === "error" ? (
            <div
              className="flex items-center justify-between gap-3"
              data-testid={`${idPrefix}-ref-error`}
            >
              <p className="text-caption text-danger">{vaultInventory.message}</p>
              <Button type="button" variant="ghost" size="xs" onClick={vaultInventory.retry}>
                Retry
              </Button>
            </div>
          ) : refs.length > 0 ? (
            <NativeSelect
              id={`${idPrefix}-ref`}
              className="font-mono"
              value={binding.vaultRef}
              onChange={event => onChange({ ...binding, vaultRef: event.target.value })}
              data-testid={`${idPrefix}-ref-select`}
            >
              {refs.map(ref => (
                <NativeSelectOption key={ref} value={ref}>
                  {ref}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          ) : (
            <p className="text-caption text-muted" data-testid={`${idPrefix}-ref-empty`}>
              No existing Vault refs in the mcp namespace for this scope.
            </p>
          )}
          <p className="mt-1.5 text-caption text-muted">
            This path selects an existing credential only. Choose Write a new value to create a new
            canonical binding.
          </p>
        </div>
      )}
    </div>
  );
}
