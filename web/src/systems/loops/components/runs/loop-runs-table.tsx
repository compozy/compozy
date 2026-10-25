import type { ComponentProps } from "react";

import { cn, ListGroupHeader, Table, TableBody } from "@compozy/ui";
import type { ProfileOwner, ProfileOwnerLabel } from "@/systems/profiles";
import type { WorkspaceBadgeIdentity } from "@/systems/workspace";

import type { LoopRunGroup } from "../../lib/loop-runs-view";
import { LoopRunRow } from "./loop-run-row";
import { RosterHead } from "./loop-runs-roster";
import { rosterColumns } from "./loop-runs-roster-columns";

interface LoopRunsTableProps extends Omit<ComponentProps<"section">, "children"> {
  group: LoopRunGroup;
  /** Resolves each run's owner. Absent in a scoped list — no tags there. */
  ownerOf?: (run: ProfileOwnerLabel) => ProfileOwner;
  /** Resolves each run's project under All projects; absent in a one-project list. */
  projectOf?: (workspaceId: string) => WorkspaceBadgeIdentity;
  nowMs: number;
}

/**
 * One roster group (Needs you / Active / Recent) as a labeled table.
 *
 * The heading is the shared list-group header plus a count: no glyph, because an
 * icon beside every group heading is decoration rather than wayfinding. Groups arrive
 * already ranked by the daemon, so this renders the order it is handed.
 */
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
