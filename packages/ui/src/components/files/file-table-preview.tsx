"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { parseDelimited } from "./delimited-text";
import { useTableVirtualizer } from "./hooks/use-table-virtualizer";

export interface FileTablePreviewProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** Delimited text; the first record is the header row. */
  text: string;
  /** Field separator: `,` for CSV, a tab for TSV. */
  delimiter: string;
  /** Rendered instead of the table when the text holds no records. */
  empty?: React.ReactNode;
}

const ROW_HEIGHT = 28;
const SAMPLE_ROWS = 200;
// A `text-xs` monospace glyph is about 7.2px wide; the estimate rounds up so short cells never clip.
const CHAR_WIDTH = 7.5;
const MIN_COLUMN = 64;
const MAX_COLUMN = 320;
const CELL = "truncate border-r border-b border-line-soft px-2 leading-7";

/** Column widths from the header and a sample of rows, so scrolling never reflows the grid. */
function columnWidths(header: readonly string[], rows: readonly string[][], count: number) {
  const widths: number[] = [];
  for (let column = 0; column < count; column += 1) {
    let chars = (header[column] ?? "").length;
    for (const row of rows.slice(0, SAMPLE_ROWS)) {
      chars = Math.max(chars, (row[column] ?? "").length);
    }
    widths.push(Math.min(MAX_COLUMN, Math.max(MIN_COLUMN, Math.ceil(chars * CHAR_WIDTH) + 18)));
  }
  return widths;
}

/**
 * Read-only table for CSV/TSV text: a sticky header, row numbers, and virtualized rows, so a
 * file with tens of thousands of records scrolls without rendering them all.
 */
function FileTablePreview({
  text,
  delimiter,
  empty = null,
  className,
  ...props
}: FileTablePreviewProps) {
  const table = React.useMemo(() => parseDelimited(text, delimiter), [text, delimiter]);
  const widths = React.useMemo(
    () => columnWidths(table.header, table.rows, table.columnCount),
    [table]
  );
  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const { items, totalSize } = useTableVirtualizer(table.rows.length, scrollRef, ROW_HEIGHT);
  if (table.columnCount === 0) return empty;
  const gutter = Math.max(40, Math.ceil(String(table.rows.length).length * CHAR_WIDTH) + 20);
  const template = `${gutter}px ${widths.map(width => `${width}px`).join(" ")}`;
  const columns = Array.from({ length: table.columnCount }, (_, column) => column);
  return (
    <div
      ref={scrollRef}
      data-slot="file-table-preview"
      data-row-count={table.rows.length}
      data-column-count={table.columnCount}
      className={cn("min-h-0 flex-1 overflow-auto bg-canvas font-mono text-xs", className)}
      {...props}
    >
      {/* A real table laid out as a grid, so rows can be positioned for virtualization. */}
      <table aria-rowcount={table.rows.length + 1} className="grid w-max min-w-full">
        <thead className="sticky top-0 z-10 grid">
          <tr
            aria-rowindex={1}
            className="grid bg-canvas-soft font-medium text-muted-foreground"
            style={{ gridTemplateColumns: template }}
          >
            <th aria-label="Row" className={cn(CELL, "text-right")} />
            {columns.map(column => (
              <th
                key={column}
                scope="col"
                title={table.header[column]}
                className={cn(CELL, "text-left font-medium")}
              >
                {table.header[column] ?? ""}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="relative grid" style={{ height: totalSize }}>
          {items.map(item => {
            const row = table.rows[item.index] ?? [];
            return (
              <tr
                key={item.key}
                aria-rowindex={item.index + 2}
                data-slot="file-table-preview-row"
                className="absolute inset-x-0 grid text-fg"
                style={{
                  gridTemplateColumns: template,
                  height: ROW_HEIGHT,
                  transform: `translateY(${item.start}px)`,
                }}
              >
                <th
                  scope="row"
                  className={cn(CELL, "text-right font-normal text-muted-foreground tabular-nums")}
                >
                  {item.index + 1}
                </th>
                {columns.map(column => (
                  <td key={column} title={row[column]} className={CELL}>
                    {row[column] ?? ""}
                  </td>
                ))}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

export { FileTablePreview };
