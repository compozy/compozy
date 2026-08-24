import * as React from "react";

import {
  IDENTITY_SURFACE_VALUE,
  identityColorsFor,
  identityInkOn,
  identitySurfaceRgb,
} from "../../lib/identity-palette";
import type { SymbolEmojiOption, SymbolIconOption, SymbolKind } from "../../lib/symbol-palette";
import { cn } from "../../lib/utils";
import { KindIcon } from "./kind-icon";
import type { KindIconRegistry } from "./kind-icon-registry";

export interface SymbolPickerGridProps extends Omit<React.ComponentProps<"div">, "onSelect"> {
  kind: SymbolKind;
  icons: readonly SymbolIconOption[];
  iconRegistry: KindIconRegistry;
  emojis: readonly SymbolEmojiOption[];
  /** Currently chosen value within this tab, or "" when the other tab owns it. */
  selected: string;
  onSelect: (value: string) => void;
  color: string;
  surface?: string;
  label: string;
  emptyMessage: React.ReactNode;
}

interface Cell {
  value: string;
  label: string;
}

/**
 * Reads the column count out of layout rather than guessing it.
 *
 * The grid uses `auto-fill`, so the number of columns is a function of the
 * rendered width. Arrow-key navigation has to follow what the operator sees,
 * and the first row break is the only honest source for that.
 */
function columnCount(container: HTMLElement | null): number {
  if (container === null) return 1;
  const cells = Array.from(container.children) as HTMLElement[];
  if (cells.length < 2) return Math.max(cells.length, 1);
  const firstTop = cells[0].offsetTop;
  const wrapAt = cells.findIndex(cell => cell.offsetTop > firstTop);
  return wrapAt === -1 ? cells.length : wrapAt;
}

function nextIndex(key: string, index: number, total: number, columns: number): number | null {
  switch (key) {
    case "ArrowRight":
      return Math.min(index + 1, total - 1);
    case "ArrowLeft":
      return Math.max(index - 1, 0);
    case "ArrowDown":
      return Math.min(index + columns, total - 1);
    case "ArrowUp":
      return Math.max(index - columns, 0);
    case "Home":
      return 0;
    case "End":
      return total - 1;
    default:
      return null;
  }
}

function cellsFor(
  kind: SymbolKind,
  icons: readonly SymbolIconOption[],
  emojis: readonly SymbolEmojiOption[]
): Cell[] {
  if (kind === "emoji") {
    return emojis.map(emoji => ({ value: emoji.value, label: emoji.label }));
  }
  return icons.map(icon => ({ value: icon.name, label: icon.label ?? icon.name }));
}

export function SymbolPickerGrid({
  className,
  kind,
  icons,
  iconRegistry,
  emojis,
  selected,
  onSelect,
  color,
  surface = IDENTITY_SURFACE_VALUE,
  label,
  emptyMessage,
  id,
  ...props
}: SymbolPickerGridProps) {
  const containerRef = React.useRef<HTMLDivElement>(null);
  const cells = cellsFor(kind, icons, emojis);
  const selectedIndex = cells.findIndex(cell => cell.value === selected);
  // Null means "wherever the selection is". Arrow keys pin a cursor that is free
  // to leave the selection; picking re-anchors it by clearing back to null.
  const [cursor, setCursor] = React.useState<number | null>(null);
  const anchor = selectedIndex === -1 ? 0 : selectedIndex;
  const active = Math.min(Math.max(cursor ?? anchor, 0), Math.max(cells.length - 1, 0));

  const gridId = id ?? React.useId();
  const optionId = (index: number) => `${gridId}-option-${index}`;
  // Emoji carry their own ink; icons are re-inked so the pairing is judged live.
  const ink = identityInkOn(color, identitySurfaceRgb(surface)).fg;
  const { bg: plate, fg: selectedInk } = identityColorsFor(color, surface);

  const handleSelect = (value: string) => {
    setCursor(null);
    onSelect(value);
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      const cell = cells[active];
      if (cell) {
        event.preventDefault();
        handleSelect(cell.value);
      }
      return;
    }
    const target = nextIndex(event.key, active, cells.length, columnCount(containerRef.current));
    if (target === null) return;
    event.preventDefault();
    setCursor(target);
  };

  if (cells.length === 0) {
    return (
      <p className="px-1 py-3 text-small-body text-subtle" data-slot="symbol-picker-empty">
        {emptyMessage}
      </p>
    );
  }

  return (
    <div
      ref={containerRef}
      role="listbox"
      aria-label={label}
      aria-activedescendant={optionId(active)}
      tabIndex={0}
      id={gridId}
      data-slot="symbol-picker-grid"
      onKeyDown={handleKeyDown}
      className={cn(
        "grid max-h-56 grid-cols-[repeat(auto-fill,minmax(var(--size-symbol-picker-cell),1fr))] gap-0.5 overflow-y-auto rounded-md outline-none focus-visible:shadow-focus-ring",
        className
      )}
      {...props}
    >
      {cells.map((cell, index) => {
        const isSelected = cell.value === selected;
        return (
          <div
            key={cell.value}
            id={optionId(index)}
            role="option"
            aria-selected={isSelected}
            aria-label={cell.label}
            data-slot="symbol-picker-option"
            data-active={index === active ? "true" : undefined}
            onClick={() => handleSelect(cell.value)}
            style={
              isSelected
                ? { backgroundColor: plate, color: kind === "emoji" ? undefined : selectedInk }
                : { color: kind === "emoji" ? undefined : ink }
            }
            className={cn(
              "grid aspect-square cursor-pointer place-items-center rounded-xs text-small-body transition-colors",
              "hover:bg-row-hover data-[active=true]:bg-row-selected"
            )}
          >
            {kind === "emoji" ? (
              <span aria-hidden="true">{cell.value}</span>
            ) : (
              <KindIcon kind={cell.value} registry={iconRegistry} className="text-current" />
            )}
          </div>
        );
      })}
    </div>
  );
}
