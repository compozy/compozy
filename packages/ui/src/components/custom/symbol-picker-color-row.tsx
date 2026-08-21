import * as React from "react";

import { normalizeHexColor, type SymbolSwatch } from "../../lib/symbol-palette";
import { cn } from "../../lib/utils";
import { FieldError } from "../field";
import { InputGroup, InputGroupAddon, InputGroupInput } from "../input-group";

export interface SymbolPickerColorRowProps extends Omit<React.ComponentProps<"div">, "onChange"> {
  color: string;
  onChange: (next: string) => void;
  onValidityChange?: (valid: boolean) => void;
  swatches: readonly SymbolSwatch[];
  swatchesLabel: string;
  customLabel: string;
  invalidMessage: string;
}

/** Keeps invalid drafts separate from the last valid identity color. */
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
        <div role="listbox" aria-label={swatchesLabel} className="flex items-center gap-1.5">
          {swatches.map(swatch => {
            const isSelected = swatch.value.toLowerCase() === color.toLowerCase();
            return (
              <button
                key={swatch.value}
                type="button"
                role="option"
                aria-selected={isSelected}
                aria-label={swatch.label}
                data-slot="symbol-picker-swatch"
                onClick={() => {
                  setDraft(null);
                  onValidityChange?.(true);
                  onChange(swatch.value);
                }}
                style={{ backgroundColor: swatch.value }}
                className={cn(
                  "size-5 shrink-0 rounded-full transition-transform outline-none",
                  "hover:scale-110 focus-visible:shadow-focus-ring motion-reduce:hover:scale-100",
                  isSelected && "ring-2 ring-fg-strong ring-offset-2 ring-offset-canvas-soft"
                )}
              />
            );
          })}
        </div>
        <span className="flex-1" aria-hidden="true" />
        <InputGroup className="h-7 w-[7.5rem]">
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
