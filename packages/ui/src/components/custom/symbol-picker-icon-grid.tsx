import * as React from "react";

import {
  IDENTITY_SURFACE_VALUE,
  identityColorsFor,
  identityInkOn,
  identitySurfaceRgb,
} from "../../lib/identity-palette";
import type { SymbolIconOption } from "../../lib/symbol-palette";
import { cn } from "../../lib/utils";
import { Spinner } from "../spinner";
import { useSymbolPickerIconGrid } from "./hooks/use-symbol-picker-icon-grid";
import { SpriteIcon } from "./sprite-icon";

export interface SymbolPickerIconGridProps extends Omit<React.ComponentProps<"div">, "onSelect"> {
  icons: readonly SymbolIconOption[];
  /** Sprite whose `<symbol>` ids are the icon names in `icons`. */
  spriteUrl: string;
  /** Currently chosen icon, or "" when an emoji owns the symbol. */
  selected: string;
  onSelect: (value: string) => void;
  color: string;
  surface?: string;
  label: string;
  emptyMessage: React.ReactNode;
  loading?: boolean;
  loadingLabel: string;
}

export function SymbolPickerIconGrid({
  className,
  icons,
  spriteUrl,
  selected,
  onSelect,
  color,
  surface = IDENTITY_SURFACE_VALUE,
  label,
  emptyMessage,
  loading = false,
  loadingLabel,
  id,
  ...props
}: SymbolPickerIconGridProps) {
  const [attachViewport, grid] = useSymbolPickerIconGrid(icons, selected, onSelect, id);
  const ink = identityInkOn(color, identitySurfaceRgb(surface)).fg;
  const { bg: plate, fg: selectedInk } = identityColorsFor(color, surface);

  if (loading) {
    return (
      <div
        role="status"
        aria-label={loadingLabel}
        data-slot="symbol-picker-loading"
        className="grid h-symbol-picker-grid place-items-center"
      >
        <Spinner className="size-4 text-subtle" />
      </div>
    );
  }

  if (icons.length === 0) {
    return (
      <p className="px-1 py-3 text-small-body text-subtle" data-slot="symbol-picker-empty">
        {emptyMessage}
      </p>
    );
  }

  const { metrics } = grid;
  return (
    <div
      ref={attachViewport}
      role="listbox"
      aria-label={label}
      aria-activedescendant={grid.optionId(grid.active)}
      tabIndex={0}
      id={grid.gridId}
      data-slot="symbol-picker-grid"
      onKeyDown={grid.handleKeyDown}
      onScroll={grid.handleScroll}
      className={cn(
        "max-h-symbol-picker-grid overflow-y-auto rounded-md outline-none focus-visible:shadow-focus-ring",
        className
      )}
      {...props}
    >
      <div
        style={
          metrics
            ? { height: grid.totalRows * grid.rowHeight - metrics.gapY, position: "relative" }
            : undefined
        }
      >
        <div
          style={{
            display: "grid",
            gridTemplateColumns: `repeat(${metrics ? grid.columns : "auto-fill"}, minmax(var(--size-symbol-picker-cell), 1fr))`,
            gap: metrics ? `${metrics.gapY}px ${metrics.gapX}px` : undefined,
            ...(metrics
              ? {
                  position: "absolute",
                  insetInline: 0,
                  top: Math.floor(grid.firstCell / grid.columns) * grid.rowHeight,
                }
              : {}),
          }}
          className={metrics ? undefined : "gap-0.5"}
        >
          {icons.slice(grid.firstCell, grid.lastCell).map((icon, offset) => {
            const index = grid.firstCell + offset;
            const isSelected = icon.name === selected;
            return (
              <div
                key={icon.name}
                id={grid.optionId(index)}
                role="option"
                aria-selected={isSelected}
                aria-label={icon.label ?? icon.name}
                data-slot="symbol-picker-option"
                data-active={index === grid.active ? "true" : undefined}
                onClick={() => grid.handleSelect(icon.name)}
                style={isSelected ? { backgroundColor: plate, color: selectedInk } : { color: ink }}
                className={cn(
                  "grid aspect-square cursor-pointer place-items-center rounded-xs transition-colors",
                  "hover:bg-row-hover data-[active=true]:bg-row-selected"
                )}
              >
                <SpriteIcon spriteUrl={spriteUrl} name={icon.name} className="text-current" />
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
