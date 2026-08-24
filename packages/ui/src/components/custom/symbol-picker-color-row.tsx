import * as React from "react";

import { normalizeHexColor, type SymbolSwatch } from "../../lib/symbol-palette";
import { cn } from "../../lib/utils";
import { FieldError } from "../field";
import { InputGroup, InputGroupAddon, InputGroupInput } from "../input-group";
import { useSwatchPalette } from "./hooks/use-swatch-palette";

export interface SymbolPickerColorRowProps extends Omit<React.ComponentProps<"div">, "onChange"> {
  color: string;
  onChange: (next: string) => void;
  onValidityChange?: (valid: boolean) => void;
  swatches: readonly SymbolSwatch[];
  swatchesLabel: string;
  customLabel: string;
  invalidMessage: string;
}

export function SymbolPickerColorRow({
  className,
  color,
  onChange,
  onValidityChange,
  swatches,
  swatchesLabel,
  customLabel,
  invalidMessage,
  ...props
}: SymbolPickerColorRowProps) {
  const [draft, setDraft] = React.useState<string | null>(null);
  const errorId = React.useId();
  const value = draft ?? color.replace(/^#/, "");
  const invalid = draft !== null && normalizeHexColor(draft) === null;
  const palette = useSwatchPalette(swatches, color, swatchValue => {
    setDraft(null);
    onValidityChange?.(true);
    onChange(swatchValue);
  });

  const handleChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const next = event.target.value;
    setDraft(next);
    const normalized = normalizeHexColor(next);
    onValidityChange?.(normalized !== null);
    if (normalized !== null) onChange(normalized);
  };

  return (
    <div className={cn("flex flex-col gap-1.5", className)} {...props}>
      <div className="flex items-center gap-2.5">
        <div
          role="listbox"
          aria-label={swatchesLabel}
          aria-activedescendant={swatches.length > 0 ? palette.swatchId(palette.active) : undefined}
          tabIndex={0}
          id={palette.listboxId}
          onKeyDown={palette.handleKeyDown}
          className="flex items-center gap-1.5 rounded-md outline-none focus-visible:shadow-focus-ring"
        >
          {swatches.map((swatch, index) => {
            const isSelected = index === palette.selectedIndex;
            return (
              <span
                key={swatch.value}
                id={palette.swatchId(index)}
                role="option"
                aria-selected={isSelected}
                aria-label={swatch.label}
                data-slot="symbol-picker-swatch"
                data-active={index === palette.active ? "true" : undefined}
                onClick={() => palette.pick(swatch.value)}
                style={{ backgroundColor: swatch.value }}
                className={cn(
                  "size-5 shrink-0 cursor-pointer rounded-full transition-transform",
                  "hover:scale-110 motion-reduce:hover:scale-100",
                  "data-[active=true]:ring-2 data-[active=true]:ring-fg data-[active=true]:ring-offset-2 data-[active=true]:ring-offset-canvas-soft",
                  isSelected && "ring-2 ring-fg-strong ring-offset-2 ring-offset-canvas-soft"
                )}
              />
            );
          })}
        </div>
        <span className="flex-1" aria-hidden="true" />
        <InputGroup className="h-search w-symbol-picker-color-input">
          <InputGroupAddon>
            <span aria-hidden="true">#</span>
          </InputGroupAddon>
          <InputGroupInput
            value={value}
            onChange={handleChange}
            aria-label={customLabel}
            aria-invalid={invalid}
            aria-describedby={invalid ? errorId : undefined}
            data-slot="symbol-picker-hex"
            className="font-mono text-badge"
          />
        </InputGroup>
      </div>
      {invalid ? <FieldError id={errorId}>{invalidMessage}</FieldError> : null}
    </div>
  );
}
