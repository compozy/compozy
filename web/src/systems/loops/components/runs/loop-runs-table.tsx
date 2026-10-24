import type { ComponentProps } from "react";

import {
  cn,
  ListGroupHeader,
  SkeletonText,
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
  TableRowSkeleton,
  type TableRowSkeletonColumn,
} from "@compozy/ui";
import type { ProfileOwner, ProfileOwnerLabel } from "@/systems/profiles";
import type { WorkspaceBadgeIdentity } from "@/systems/workspace";

import type { LoopRunGroup } from "../../lib/loop-runs-view";
import { LoopRunRow } from "./loop-run-row";

interface LoopRunsTableProps extends Omit<ComponentProps<"section">, "children"> {
  group: LoopRunGroup;
  /** Resolves each run's owner. Absent in a scoped list — no tags there. */
  ownerOf?: (run: ProfileOwnerLabel) => ProfileOwner;
  /** Resolves each run's project under All projects; absent in a one-project list. */
  projectOf?: (workspaceId: string) => WorkspaceBadgeIdentity;
  nowMs: number;
}

/**
 * The roster's closed column set; a further column is a design decision, not a
 * typo. Project is the All projects listing's identity column (`_uiux.md` S5).
 */
type LoopRunColumnKey = "loop" | "project" | "status" | "progress" | "started" | "duration";

interface LoopRunColumn {
  key: LoopRunColumnKey;
  label: string;
  className?: string;
}

/** Gens / Best / Budget are demoted to the run page. */
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
 * The heading is the shared list-group header plus a count: no glyph, because an
 * icon beside every group heading is decoration rather than wayfinding. Groups arrive
 * already ranked by the daemon, so this renders the order it is handed.
 */
const PROJECT_COLUMN: LoopRunColumn = { key: "project", label: "Project" };

function rosterColumns(withProject: boolean): readonly LoopRunColumn[] {
  return withProject ? [COLUMNS[0], PROJECT_COLUMN, ...COLUMNS.slice(1)] : COLUMNS;
}

/** The column header every group shares: the real labels, so the skeleton prints them too. */
function RosterHead({ columns }: { columns: readonly LoopRunColumn[] }) {
  return (
    <TableHeader>
      <TableRow className="hover:bg-transparent">
        {columns.map(column => (
          <TableHead className={column.className} key={column.key}>
            {column.label}
          </TableHead>
        ))}
      </TableRow>
    </TableHeader>
  );
}

export function LoopRunsTable({
  group,
  ownerOf,
  projectOf,
  nowMs,
  className,
  ...props
}: LoopRunsTableProps) {
  const headingId = `loop-runs-group-${group.id}-heading`;
  const columns = rosterColumns(projectOf !== undefined);
  return (
    <section
      aria-labelledby={headingId}
      className={cn(className)}
      data-group={group.id}
      data-testid={`loop-runs-group-${group.id}`}
      {...props}
    >
      <div className="overflow-hidden rounded-lg bg-card shadow-card">
        <ListGroupHeader
          count={<span data-testid="loop-runs-count">{group.rows.length}</span>}
          label={<span id={headingId}>{group.label}</span>}
        />
        <Table>
          <RosterHead columns={columns} />
          <TableBody>
            {group.rows.map(row => (
              <LoopRunRow
                key={row.run.id}
                nowMs={nowMs}
                owner={ownerOf?.(row.run)}
                project={projectOf?.(row.run.workspace_id)}
                row={row}
              />
            ))}
          </TableBody>
        </Table>
      </div>
    </section>
  );
}

/** Placeholder widths per column, close to what the cells print so the columns keep their size. */
const SKELETON_CELL: Record<LoopRunColumnKey, TableRowSkeletonColumn> = {
  loop: { className: "w-48" },
  project: { className: "w-28" },
  status: { className: "w-24" },
  progress: { className: "w-36" },
  started: { className: "w-32" },
  duration: { className: "w-14" },
};

/**
 * One roster group while the runs are being read: the group card and the real column header,
 * with placeholder rows in the table body. Presentational; the caller owns the `SkeletonRegion`
 * (or hides it, when a degraded notice already speaks for the area).
 */
export function LoopRunsTableSkeleton({
  withProject = false,
  rows = 6,
  ...props
}: Omit<ComponentProps<"section">, "children"> & { withProject?: boolean; rows?: number }) {
  const columns = rosterColumns(withProject);
  return (
    <section {...props}>
      <div className="overflow-hidden rounded-lg bg-card shadow-card">
        <ListGroupHeader
          count={<SkeletonText className="w-3" />}
          label={<SkeletonText className="w-16" />}
        />
        <Table>
          <RosterHead columns={columns} />
          <TableBody>
            <TableRowSkeleton
              columns={columns.map(column => SKELETON_CELL[column.key])}
              count={rows}
            />
          </TableBody>
        </Table>
      </div>
    </section>
  );
}
