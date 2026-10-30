"use client";

import { Autocomplete as AutocompletePrimitive } from "@base-ui/react/autocomplete";
import { Folder } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";
import { Button } from "../button";
import { InputGroup, InputGroupAddon, InputGroupInput } from "../input-group";

export interface DirectoryPickerSuggestion {
  /** Text the field takes when the suggestion is chosen — usually the path plus a separator. */
  value: string;
  /** Folder name shown in the list. */
  label: string;
}

export interface DirectoryPickerInputProps extends Omit<
  React.ComponentProps<"input">,
  "value" | "defaultValue" | "onChange" | "type" | "children"
> {
  value: string;
  onValueChange: (value: string) => void;
  /** Completions for the current value, already filtered by the caller. */
  suggestions: readonly DirectoryPickerSuggestion[];
  /** Opens a native folder chooser. When absent, no Browse button renders. */
  onBrowse?: () => void;
  browseLabel?: string;
  browsePending?: boolean;
  invalid?: boolean;
  /** Accessible name of the suggestion list. */
  suggestionsLabel?: string;
  /** Classes for the outer row that holds the field and the Browse button. */
  rootClassName?: string;
}

/**
 * A path field for choosing a folder: a mono input with keyboard completion
 * (↑/↓ move, Enter or click choose, Tab completes and keeps the list open to
 * drill down, Esc closes) and an optional Browse… button for a native chooser.
 *
 * Presentational: the caller owns the value, computes the suggestions, and
 * reports validation through `invalid` plus its own field message.
 */
function DirectoryPickerInput({
  value,
  onValueChange,
  suggestions,
  onBrowse,
  browseLabel = "Browse…",
  browsePending = false,
  invalid = false,
  suggestionsLabel = "Folder suggestions",
  rootClassName,
  className,
  disabled,
  onKeyDown,
  ...inputProps
}: DirectoryPickerInputProps) {
  const [open, setOpen] = React.useState(false);
  const [highlighted, setHighlighted] = React.useState<DirectoryPickerSuggestion | undefined>();
  const showList = open && suggestions.length > 0;

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    onKeyDown?.(event);
    if (event.defaultPrevented) return;
    if (event.key !== "Tab" || event.shiftKey || !showList) return;
    const next = highlighted ?? suggestions[0];
    if (!next || next.value === value) return;
    event.preventDefault();
    onValueChange(next.value);
    setHighlighted(undefined);
  };

  return (
    <div className={cn("flex min-w-0 items-center gap-2", rootClassName)}>
      <AutocompletePrimitive.Root
        disabled={disabled}
        filter={null}
        items={suggestions}
        itemToStringValue={item => item.value}
        onItemHighlighted={item => setHighlighted(item)}
        onOpenChange={setOpen}
        onValueChange={next => onValueChange(next)}
        open={showList}
        value={value}
      >
        {/* The group, not the bare input, anchors the suggestion list. */}
        <AutocompletePrimitive.InputGroup
          render={
            <InputGroup
              className={cn(
                "min-w-0 flex-1 has-[[data-slot=input-group-control]:focus-visible]:shadow-focus-ring",
                className
              )}
              data-slot="directory-picker-input"
            />
          }
        >
          <InputGroupAddon align="inline-start">
            <Folder aria-hidden="true" className="size-3.5 text-subtle" />
          </InputGroupAddon>
          <AutocompletePrimitive.Input
            aria-invalid={invalid || undefined}
            autoCapitalize="off"
            autoComplete="off"
            autoCorrect="off"
            disabled={disabled}
            onKeyDown={handleKeyDown}
            render={
              <InputGroupInput className="font-mono text-form-input focus-visible:shadow-none" />
            }
            spellCheck={false}
            {...inputProps}
          />
        </AutocompletePrimitive.InputGroup>
        <AutocompletePrimitive.Portal>
          <AutocompletePrimitive.Positioner align="start" className="isolate z-50" sideOffset={6}>
            <AutocompletePrimitive.Popup
              className="max-h-(--available-height) w-(--anchor-width) max-w-(--available-width) origin-(--transform-origin) overflow-hidden rounded-lg bg-popover text-fg shadow-pop duration-fast data-closed:animate-out data-closed:fade-out-0 data-open:animate-in data-open:fade-in-0"
              data-slot="directory-picker-suggestions"
            >
              <AutocompletePrimitive.List
                aria-label={suggestionsLabel}
                className="no-scrollbar max-h-60 scroll-py-1 overflow-y-auto overscroll-contain p-1"
              >
                {(item: DirectoryPickerSuggestion) => (
                  <AutocompletePrimitive.Item
                    className="flex min-h-8 cursor-default items-center gap-2 rounded-md px-2.5 py-1.5 outline-hidden select-none data-highlighted:bg-surface-2"
                    data-slot="directory-picker-suggestion"
                    key={item.value}
                    value={item}
                  >
                    <Folder aria-hidden="true" className="size-3.5 flex-none text-muted" />
                    <span className="truncate font-mono text-mono-id text-fg">{item.label}</span>
                  </AutocompletePrimitive.Item>
                )}
              </AutocompletePrimitive.List>
            </AutocompletePrimitive.Popup>
          </AutocompletePrimitive.Positioner>
        </AutocompletePrimitive.Portal>
      </AutocompletePrimitive.Root>
      {onBrowse ? (
        <Button
          data-slot="directory-picker-browse"
          disabled={disabled || browsePending}
          onClick={onBrowse}
          type="button"
          variant="secondary"
        >
          {browseLabel}
        </Button>
      ) : null}
    </div>
  );
}

export { DirectoryPickerInput };
