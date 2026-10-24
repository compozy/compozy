import {
  SkeletonRegion,
  Table,
  TableBody,
  TableRowSkeleton,
  type TableRowSkeletonColumn,
} from "@compozy/ui";

import { AgentSessionsTableHead } from "./agent-sessions-table-head";

const SESSION_SKELETON_ROWS = 5;
const SESSION_SKELETON_COLUMNS: readonly TableRowSkeletonColumn[] = [
  { className: "w-48" },
  { className: "w-20" },
  { align: "end", className: "w-12" },
  { align: "end", className: "w-16" },
  { align: "end", className: "w-5" },
];

/** The sessions table while its rows load: the real header and cell grid, placeholder rows. */
export function AgentSessionsSkeleton() {
  return (
    <SkeletonRegion
      className="flex flex-col"
      data-testid="agent-sessions-loading"
      label="Loading sessions"
    >
      <div className="overflow-x-auto">
        <Table>
          <AgentSessionsTableHead />
          <TableBody>
            <TableRowSkeleton columns={SESSION_SKELETON_COLUMNS} count={SESSION_SKELETON_ROWS} />
          </TableBody>
        </Table>
      </div>
    </SkeletonRegion>
  );
}
