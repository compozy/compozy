import { useState } from "react";

import {
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  Eyebrow,
  Input,
} from "@compozy/ui";

import type { LoopEntityCatalog, LoopEntityOption } from "../../lib/loop-input-catalogs";

export interface LoopCatalogValueSelectProps {
  catalog: LoopEntityCatalog;
  label: string;
  value: string;
  controlId: string;
  testId: string;
  disabled?: boolean;
  invalid?: boolean;
  describedBy?: string;
  allowManual?: boolean;
  onChange: (value: string) => void;
}

function catalogTriggerText(
  catalog: LoopEntityCatalog,
  label: string,
  value: string,
  selected: LoopEntityOption | undefined
): string {
  const current = selected?.label ?? value;
  if (current) return current;
  return catalog.loading ? `Loading ${label}s…` : `Select ${label}`;
}

function catalogEmptyText(catalog: LoopEntityCatalog, label: string): string {
  return catalog.loading ? `Loading ${label}s…` : `No ${label}s match your search.`;
}

type LoopCatalogManualInputProps = Omit<LoopCatalogValueSelectProps, "catalog" | "allowManual"> & {
  error: string;
  missing: boolean;
};

/** Exact-value fallback when the catalog failed to load and manual entry is allowed. */
function LoopCatalogManualInput({
  error,
  missing,
  label,
  value,
  controlId,
  testId,
  disabled,
  invalid,
  describedBy,
  onChange,
}: LoopCatalogManualInputProps) {
  return (
    <div className="flex flex-col gap-1.5">
      <Input
        aria-describedby={describedBy}
        aria-invalid={invalid || missing || undefined}
        className="font-mono"
        data-testid={testId}
        disabled={disabled}
        id={controlId}
        onChange={event => onChange(event.target.value)}
        placeholder={`Enter exact ${label}`}
        type="text"
        value={value}
      />
      <p className="text-form-hint text-warning" role="status">
        {error} Enter the exact value.
      </p>
    </div>
  );
}

export function LoopCatalogValueSelect({
  catalog,
  label,
  value,
  controlId,
  testId,
  disabled,
  invalid,
  describedBy,
  allowManual = true,
  onChange,
}: LoopCatalogValueSelectProps) {
  const [open, setOpen] = useState(false);
  const selected = catalog.options.find(option => option.value === value);
  const missing = value !== "" && selected === undefined;

  if (catalog.error && allowManual) {
    return (
      <LoopCatalogManualInput
        controlId={controlId}
        describedBy={describedBy}
        disabled={disabled}
        error={catalog.error}
        invalid={invalid}
        label={label}
        missing={missing}
        onChange={onChange}
        testId={testId}
        value={value}
      />
    );
  }

  const unavailable = catalog.loading && catalog.options.length === 0;
  return (
    <CommandSelect open={open} onOpenChange={setOpen}>
      <CommandSelectTrigger
        aria-busy={catalog.loading || undefined}
        aria-describedby={describedBy}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-invalid={invalid || missing || undefined}
        className="w-full justify-between"
        data-testid={testId}
        disabled={disabled || unavailable}
        id={controlId}
        selected={Boolean(selected) || missing}
      >
        <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
          <span className="min-w-0 flex-1 truncate font-mono text-small-body text-fg">
            {catalogTriggerText(catalog, label, value, selected)}
          </span>
          {missing ? <Eyebrow className="shrink-0 text-warning">Not available</Eyebrow> : null}
        </span>
      </CommandSelectTrigger>
      <CommandSelectShell className="min-w-64" inputPlaceholder={`Search ${label}s...`}>
        <CommandList>
          <CommandEmpty>{catalogEmptyText(catalog, label)}</CommandEmpty>
          <CommandSelectGroup>
            {catalog.options.map(option => (
              <CommandItem
                data-checked={option.value === value ? "true" : "false"}
                key={option.value}
                onSelect={() => {
                  onChange(option.value);
                  setOpen(false);
                }}
                value={`${option.label} ${option.detail ?? ""}`}
              >
                <span className="min-w-0 flex-1 truncate text-small-body text-fg">
                  {option.label}
                </span>
                {option.detail ? (
                  <span className="truncate font-mono text-mono-id text-subtle">
                    {option.detail}
                  </span>
                ) : null}
              </CommandItem>
            ))}
          </CommandSelectGroup>
        </CommandList>
      </CommandSelectShell>
    </CommandSelect>
  );
}
