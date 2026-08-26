import * as React from "react";

import { useMeasure } from "../../hooks/use-measure";
import {
  IDENTITY_SURFACE_VALUE,
  identityColorsFor,
  identityInkOn,
  identitySurfaceRgb,
} from "../../lib/identity-palette";
import type { SymbolIconOption } from "../../lib/symbol-palette";
import { cn } from "../../lib/utils";
import { Spinner } from "../spinner";
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

interface GridMetrics {
  cell: number;
  gapX: number;
  gapY: number;
  columns: number;
}

/** Rows rendered above and below the viewport so scrolling never shows a gap. */
const OVERSCAN_ROWS = 2;
/** Cells rendered before metrics exist; enough to fill the first viewport. */
const BOOTSTRAP_CELLS = 200;

function metricsFrom(viewport: HTMLElement | null): GridMetrics | null {
  const block = viewport?.firstElementChild?.firstElementChild;
  const cells = block ? (Array.from(block.children) as HTMLElement[]) : [];
  const first = cells[0];
  if (!viewport || !first || first.offsetWidth === 0 || viewport.clientWidth === 0) return null;
  const cell = first.offsetWidth;
  const firstTop = first.offsetTop;
  const wrapAt = cells.findIndex(candidate => candidate.offsetTop > firstTop);
  const columns = wrapAt === -1 ? cells.length : wrapAt;
  const second = cells[1];
  const below = wrapAt === -1 ? null : cells[wrapAt];
  const gapX = second && columns > 1 ? Math.max(0, second.offsetLeft - first.offsetLeft - cell) : 0;
  const gapY = below ? Math.max(0, below.offsetTop - firstTop - first.offsetHeight) : gapX;
  return { cell, gapX, gapY, columns };
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
  const viewportRef = React.useRef<HTMLDivElement>(null);
  const [measureRef, bounds] = useMeasure<HTMLDivElement>();
  const generatedId = React.useId();
  const [metrics, setMetrics] = React.useState<GridMetrics | null>(null);
  const [scrollTop, setScrollTop] = React.useState(0);
  // Null means "wherever the selection is". Arrow keys pin a cursor that is free
  // to leave the selection; picking re-anchors it by clearing back to null.
  const [cursor, setCursor] = React.useState<number | null>(null);

  const selectedIndex = icons.findIndex(icon => icon.name === selected);
  const anchor = selectedIndex === -1 ? 0 : selectedIndex;
  const active = Math.min(Math.max(cursor ?? anchor, 0), Math.max(icons.length - 1, 0));

  const gridId = id ?? generatedId;
  const optionId = (index: number) => `${gridId}-option-${index}`;
  const ink = identityInkOn(color, identitySurfaceRgb(surface)).fg;
  const { bg: plate, fg: selectedInk } = identityColorsFor(color, surface);

  // The grid re-measures from real cells whenever the viewport width changes;
  // unmeasurable environments (jsdom, print) keep the static full render.
  React.useLayoutEffect(() => {
    setMetrics(metricsFrom(viewportRef.current));
  }, [bounds.width]);

  const rowHeight = metrics ? metrics.cell + metrics.gapY : 0;
  const columns = metrics?.columns ?? 1;
  const totalRows = metrics ? Math.ceil(icons.length / columns) : 1;

  let firstCell = 0;
  let lastCell = Math.min(icons.length, BOOTSTRAP_CELLS);
  if (metrics && rowHeight > 0) {
    const viewportHeight = viewportRef.current?.clientHeight ?? 0;
    const activeRow = Math.floor(active / columns);
    const firstRow = Math.max(
      0,
      Math.min(Math.floor(scrollTop / rowHeight) - OVERSCAN_ROWS, activeRow)
    );
    const lastRow = Math.min(
      totalRows - 1,
      Math.max(Math.ceil((scrollTop + viewportHeight) / rowHeight) + OVERSCAN_ROWS, activeRow)
    );
    firstCell = firstRow * columns;
    lastCell = Math.min(icons.length, (lastRow + 1) * columns);
  }

  // Keyboard navigation keeps the active cell inside the scrolled viewport.
  React.useLayoutEffect(() => {
    const viewport = viewportRef.current;
    if (cursor === null || !viewport || !metrics || rowHeight === 0) return;
    const rowTop = Math.floor(cursor / metrics.columns) * rowHeight;
    const rowBottom = rowTop + metrics.cell;
    if (rowTop < viewport.scrollTop) viewport.scrollTop = rowTop;
    else if (rowBottom > viewport.scrollTop + viewport.clientHeight) {
      viewport.scrollTop = rowBottom - viewport.clientHeight;
    }
  }, [cursor, metrics, rowHeight]);

  const handleSelect = (value: string) => {
    setCursor(null);
    onSelect(value);
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      const icon = icons[active];
      if (icon) {
        event.preventDefault();
        handleSelect(icon.name);
      }
      return;
    }
    const target = nextIndex(event.key, active, icons.length, columns);
    if (target === null) return;
    event.preventDefault();
    setCursor(target);
  };

  if (loading) {
    return (
      <div
        role="status"
        aria-label={loadingLabel}
        data-slot="symbol-picker-loading"
        className="grid h-(--height-symbol-picker-grid) place-items-center"
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

  return (
    <div
      ref={node => {
        viewportRef.current = node;
        measureRef(node);
      }}
      role="listbox"
      aria-label={label}
      aria-activedescendant={optionId(active)}
      tabIndex={0}
      id={gridId}
      data-slot="symbol-picker-grid"
      onKeyDown={handleKeyDown}
      onScroll={event => setScrollTop(event.currentTarget.scrollTop)}
      className={cn(
        "max-h-(--height-symbol-picker-grid) overflow-y-auto rounded-md outline-none focus-visible:shadow-focus-ring",
        className
      )}
      {...props}
    >
      <div
        style={
          metrics
            ? { height: totalRows * rowHeight - metrics.gapY, position: "relative" }
            : undefined
        }
      >
        <div
          style={{
            display: "grid",
            gridTemplateColumns: `repeat(${metrics ? columns : "auto-fill"}, minmax(var(--size-symbol-picker-cell), 1fr))`,
            gap: metrics ? `${metrics.gapY}px ${metrics.gapX}px` : undefined,
            ...(metrics
              ? {
                  position: "absolute",
                  insetInline: 0,
                  top: Math.floor(firstCell / columns) * rowHeight,
                }
              : {}),
          }}
          className={metrics ? undefined : "gap-0.5"}
        >
          {icons.slice(firstCell, lastCell).map((icon, offset) => {
            const index = firstCell + offset;
            const isSelected = icon.name === selected;
            return (
              <div
                key={icon.name}
                id={optionId(index)}
                role="option"
                aria-selected={isSelected}
                aria-label={icon.label ?? icon.name}
                data-slot="symbol-picker-option"
                data-active={index === active ? "true" : undefined}
                onClick={() => handleSelect(icon.name)}
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
