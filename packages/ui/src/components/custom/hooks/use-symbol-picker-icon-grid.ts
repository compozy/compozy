import * as React from "react";

import { useMeasure } from "../../../hooks/use-measure";
import type { SymbolIconOption } from "../../../lib/symbol-palette";

export interface SymbolPickerGridMetrics {
  cell: number;
  gapX: number;
  gapY: number;
  columns: number;
}

export interface SymbolPickerIconGridModel {
  gridId: string;
  optionId: (index: number) => string;
  active: number;
  metrics: SymbolPickerGridMetrics | null;
  rowHeight: number;
  columns: number;
  totalRows: number;
  firstCell: number;
  lastCell: number;
  handleSelect: (value: string) => void;
  handleKeyDown: (event: React.KeyboardEvent<HTMLDivElement>) => void;
  handleScroll: (event: React.UIEvent<HTMLDivElement>) => void;
}

/** Rows rendered above and below the viewport so scrolling never shows a gap. */
const OVERSCAN_ROWS = 2;
/** Cells rendered before metrics exist; enough to fill the first viewport. */
const BOOTSTRAP_CELLS = 200;

function metricsFrom(viewport: HTMLElement | null): SymbolPickerGridMetrics | null {
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

/** Windowing, roving cursor, and keyboard model for the icon grid. */
export function useSymbolPickerIconGrid(
  icons: readonly SymbolIconOption[],
  selected: string,
  onSelect: (value: string) => void,
  id?: string
): readonly [React.RefCallback<HTMLDivElement>, SymbolPickerIconGridModel] {
  const viewportRef = React.useRef<HTMLDivElement>(null);
  const [measureRef, bounds] = useMeasure<HTMLDivElement>();
  const [attachViewport] = React.useState<React.RefCallback<HTMLDivElement>>(
    () => (node: HTMLDivElement | null) => {
      viewportRef.current = node;
      measureRef(node);
    }
  );
  const generatedId = React.useId();
  const [metrics, setMetrics] = React.useState<SymbolPickerGridMetrics | null>(null);
  const [scrollTop, setScrollTop] = React.useState(0);
  // Null means "wherever the selection is". Arrow keys pin a cursor that is free
  // to leave the selection; picking re-anchors it by clearing back to null.
  const [cursor, setCursor] = React.useState<number | null>(null);

  const selectedIndex = icons.findIndex(icon => icon.name === selected);
  const anchor = selectedIndex === -1 ? 0 : selectedIndex;
  const active = Math.min(Math.max(cursor ?? anchor, 0), Math.max(icons.length - 1, 0));

  const gridId = id ?? generatedId;

  // The grid re-measures from real cells whenever the viewport size changes;
  // unmeasurable environments (jsdom, print) keep the static bootstrap render.
  React.useLayoutEffect(() => {
    setMetrics(metricsFrom(viewportRef.current));
  }, [bounds.width]);

  const rowHeight = metrics ? metrics.cell + metrics.gapY : 0;
  const columns = metrics?.columns ?? 1;
  const totalRows = metrics ? Math.ceil(icons.length / columns) : 1;

  let firstCell = 0;
  let lastCell = Math.min(icons.length, BOOTSTRAP_CELLS);
  if (metrics && rowHeight > 0) {
    const activeRow = Math.floor(active / columns);
    const firstRow = Math.max(
      0,
      Math.min(Math.floor(scrollTop / rowHeight) - OVERSCAN_ROWS, activeRow)
    );
    const lastRow = Math.min(
      totalRows - 1,
      Math.max(Math.ceil((scrollTop + bounds.height) / rowHeight) + OVERSCAN_ROWS, activeRow)
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

  const handleScroll = (event: React.UIEvent<HTMLDivElement>) => {
    setScrollTop(event.currentTarget.scrollTop);
  };

  return [
    attachViewport,
    {
      gridId,
      optionId: index => `${gridId}-option-${index}`,
      active,
      metrics,
      rowHeight,
      columns,
      totalRows,
      firstCell,
      lastCell,
      handleSelect,
      handleKeyDown,
      handleScroll,
    },
  ] as const;
}
