import { useState, type KeyboardEvent } from "react";

import { Input, cn } from "@compozy/ui";

import { filterArgOptions, type PaletteArgField } from "../lib/cmd-palette-args";

export interface PaletteArgDropdownProps {
  field: PaletteArgField;
  className: string;
  focused: boolean;
  /** Hands the input node up so a blocked submit can focus it. */
  registerNode: (node: HTMLElement | null) => void;
  onChange: (value: string) => void;
  /** ⏎ with the list closed leaves the field and submits the bar. */
  onSubmit: () => void;
}

/**
 * A dropdown argument (US-015.AC-3).
 *
 * The field itself is the combobox — the operator types to filter in the same
 * box the value lands in, rather than opening a second search. The list renders
 * in flow beneath the field, inside the palette's own frame: a floating popup
 * would sit over the footer and read as a second sheet of chrome, which the
 * artboard explicitly rules out.
 *
 * It owns no Escape rung. The ladder is panel → argument or confirmation step →
 * palette, and an open option list is not a step on it: Escape in argument mode
 * restores the search and discards what was typed, wherever the caret happens to
 * be. Swallowing it here would make one field's list behave unlike every other
 * part of the surface.
 */
export function PaletteArgDropdown({
  field,
  className,
  focused,
  registerNode,
  onChange,
  onSubmit,
}: PaletteArgDropdownProps) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const options = filterArgOptions(field, field.value);
  const clampedIndex = Math.min(activeIndex, Math.max(options.length - 1, 0));
  const active = options[clampedIndex];
  const listId = `os-palette-arg-options-${field.name}`;
  const activeOptionId = active === undefined ? undefined : `${listId}-option-${clampedIndex}`;

  const pick = (option: string) => {
    onChange(option);
    setOpen(false);
    setActiveIndex(0);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      if (options.length === 0) return;
      event.preventDefault();
      setOpen(true);
      const step = event.key === "ArrowDown" ? 1 : -1;
      setActiveIndex(current => (current + step + options.length) % options.length);
      return;
    }
    if (event.key !== "Enter") return;
    event.preventDefault();
    event.stopPropagation();
    if (open && active !== undefined) {
      pick(active);
      return;
    }
    onSubmit();
  };

  return (
    <div className="relative flex flex-col">
      <Input
        aria-activedescendant={open ? activeOptionId : undefined}
        aria-autocomplete="list"
        aria-controls={listId}
        aria-describedby={field.error === "" ? undefined : `os-palette-arg-error-${field.name}`}
        aria-expanded={open}
        aria-invalid={field.error !== "" ? true : undefined}
        autoComplete="off"
        autoFocus={focused}
        className={cn(className, field.error !== "" && "border-danger")}
        data-testid={`os-palette-arg-${field.name}`}
        id={`os-palette-arg-${field.name}`}
        placeholder={field.placeholder}
        ref={registerNode}
        role="combobox"
        value={field.value}
        onChange={event => {
          onChange(event.target.value);
          setOpen(true);
          setActiveIndex(0);
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={onKeyDown}
      />
      {open && options.length > 0 ? (
        <div
          aria-label={field.name}
          className="mt-1 flex flex-col gap-0.5 rounded-md bg-canvas-soft p-1 shadow-hairline"
          data-testid={listId}
          id={listId}
          role="listbox"
        >
          {options.map((option, index) => (
            <button
              aria-selected={index === clampedIndex}
              className={cn(
                "flex h-8 items-center rounded-sm px-2 text-left text-small-body text-fg",
                index === clampedIndex && "bg-elevated text-fg-strong"
              )}
              id={`${listId}-option-${index}`}
              key={option}
              role="option"
              // The field keeps DOM focus so the combobox contract holds: options
              // are reached with the arrow keys through `aria-activedescendant`,
              // never by tabbing to them.
              tabIndex={-1}
              type="button"
              onMouseDown={event => {
                event.preventDefault();
                pick(option);
              }}
            >
              {option}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
