import type { ComponentProps } from "react";

import {
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

import {
  rosterColumns,
  type LoopRunColumn,
  type LoopRunColumnKey,
} from "./loop-runs-roster-columns";

/** The column header every group shares: the real labels, so the skeleton prints them too. */
export function RosterHead({ columns }: { columns: readonly LoopRunColumn[] }) {
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
