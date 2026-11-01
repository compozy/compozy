import { useState } from "react";
import { ChevronDown } from "@compozy/ui/icons";

import {
  cn,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
} from "@compozy/ui";

export interface CreateDestinationOption {
  value: string;
  label: string;
}

export interface CreateDestinationPickerProps {
  /** Accessible name of the axis: "Project" or "Profile". */
  axis: string;
  options: readonly CreateDestinationOption[];
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  testId: string;
}

/**
 * Compact inline select with search for one creation axis. The trigger reads as
 * the bold value inside the sentence; Tab reaches it and typing opens the list
 * already filtered by that character.
 */
export function CreateDestinationPicker({
  axis,
  options,
  value,
  onChange,
  disabled,
  testId,
}: CreateDestinationPickerProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const selected = options.find(option => option.value === value);

  return (
    <CommandSelect
      open={open}
      onOpenChange={next => {
        setOpen(next);
        if (!next) setQuery("");
      }}
    >
      <CommandSelectTrigger
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={`${axis}: ${selected?.label ?? "none"}`}
        data-testid={testId}
        disabled={disabled}
        selected={Boolean(selected)}
        onKeyDown={event => {
          if (open || event.key.length !== 1 || event.metaKey || event.ctrlKey) return;
          setQuery(event.key);
          setOpen(true);
        }}
        className={cn(
          "inline-flex h-auto w-auto min-w-0 gap-0.5 rounded-sm border-transparent bg-transparent px-0.5 py-0 align-baseline font-medium text-fg shadow-none",
          "hover:bg-surface-2 focus-visible:border-transparent [&>svg:last-child]:hidden"
        )}
      >
        <span className="min-w-0 truncate">{selected?.label ?? "Choose"}</span>
        <ChevronDown aria-hidden="true" className="size-3 shrink-0 text-subtle" />
      </CommandSelectTrigger>
      <CommandSelectShell
        className="min-w-56"
        inputPlaceholder={`Search ${axis.toLowerCase()}s…`}
        inputProps={{
          value: query,
          onValueChange: setQuery,
          "data-testid": `${testId}-input`,
        }}
      >
        <CommandList>
          <CommandEmpty>No matches.</CommandEmpty>
          <CommandSelectGroup heading={axis}>
            {options.map(option => (
              <CommandItem
                key={option.value}
                value={`${option.label} ${option.value}`}
                data-checked={option.value === value ? "true" : "false"}
                data-testid={`${testId}-item-${option.value}`}
                onSelect={() => {
                  onChange(option.value);
                  setOpen(false);
                  setQuery("");
                }}
              >
                <span className="truncate text-small-body text-fg">{option.label}</span>
              </CommandItem>
            ))}
          </CommandSelectGroup>
        </CommandList>
      </CommandSelectShell>
    </CommandSelect>
  );
}
