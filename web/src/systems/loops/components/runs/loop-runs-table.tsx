import type { ComponentProps } from "react";

import { cn, Eyebrow, Table, TableBody, TableHead, TableHeader, TableRow } from "@compozy/ui";

import type { LoopRunGroup } from "../../lib/loop-runs-view";
import { LoopRunRow } from "./loop-run-row";

interface LoopRunsTableProps extends Omit<ComponentProps<"section">, "children"> {
  group: LoopRunGroup;
}

/** The roster's closed column set; a sixth column is a design decision, not a typo. */
type LoopRunColumnKey = "loop" | "status" | "progress" | "started" | "duration";

interface LoopRunColumn {
  key: LoopRunColumnKey;
  label: string;
  className?: string;
}

/**
 * The run projection exposes `created_at` and `last_progress_at`, so the roster
 * can state when a run started and how long it has been going — never an "Ended"
 * it does not know. Gens / Best / Budget are demoted to the run page.
 */
const COLUMNS: readonly LoopRunColumn[] = [
  { key: "loop", label: "Loop", className: "w-full" },
  { key: "status", label: "Status" },
  { key: "progress", label: "Progress" },
  { key: "started", label: "Started" },
  { key: "duration", label: "Duration" },
];

/**
 * One roster group (Needs you / Active / Recent) as a labeled table.
 *
 * The heading is a bare eyebrow plus a count: no glyph, because an icon beside
 * every group heading is decoration rather than wayfinding. Groups arrive
 * already ranked by the daemon, so this renders the order it is handed.
 */
export function LoopRunsTable({ group, className, ...props }: LoopRunsTableProps) {
  const headingId = `loop-runs-group-${group.id}-heading`;
  return (
    <section
      aria-labelledby={headingId}
      className={cn(className)}
      data-group={group.id}
      data-testid={`loop-runs-group-${group.id}`}
      {...props}
    >
      <div className="flex min-h-6 items-center gap-2 px-0.5 pb-2.5">
        <h2 className="min-w-0" id={headingId}>
          <Eyebrow className="text-subtle">{group.label}</Eyebrow>
        </h2>
        <span
          className="inline-flex h-count-chip min-w-count-chip items-center justify-center rounded-mono-badge bg-canvas-soft px-1.5 font-mono text-mono-id font-medium tabular-nums text-muted"
          data-testid="loop-runs-count"
        >
          {group.rows.length}
        </span>
      </div>
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              {COLUMNS.map(column => (
                <TableHead className={column.className} key={column.key}>
                  {column.label}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {group.rows.map(row => (
              <LoopRunRow key={row.run.id} row={row} />
            ))}
          </TableBody>
        </Table>
      </div>
    </section>
  );
}
