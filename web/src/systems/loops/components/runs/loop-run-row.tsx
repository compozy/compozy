import { Link } from "@tanstack/react-router";

import { cn, StateGlyph, TableCell, TableRow, Time } from "@compozy/ui";

import { formatClockDuration, runElapsedSeconds } from "../../lib/loop-run-usage";
import type { LoopRunRow as LoopRunRowModel } from "../../lib/loop-runs-view";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";
import {
  itemBindingSearch,
  useWindowBindingSearch,
  WorkspaceBadge,
  type WorkspaceBadgeIdentity,
} from "@/systems/workspace";

interface LoopRunRowProps {
  row: LoopRunRowModel;
  /** The run's profile, supplied only in aggregate mode. */
  owner?: ProfileOwner;
  /** The run's project, supplied only under All projects: its own column, and the link binds to it. */
  project?: WorkspaceBadgeIdentity;
  /** Display clock, so a live run's duration ticks the same as on the Loop page. */
  nowMs: number;
}

const META_CELL = "font-mono text-mono-id tabular-nums text-muted-foreground";

/**
 * One roster row: Loop · Status · Progress · Started · Duration.
 *
 * Everything the row says comes from the server-owned row model — it never
 * re-derives a status, an attention marker, or a step count from the raw run.
 * Spend (generations, best score, budget) is deliberately absent: it is demoted
 * to the run page, where there is room to say what it means.
 */
export function LoopRunRow({ row, owner, project, nowMs }: LoopRunRowProps) {
  const { run } = row;
  const windowSearch = useWindowBindingSearch();
  const linkSearch = project ? itemBindingSearch(run.workspace_id) : windowSearch;
  return (
    <TableRow
      className={cn(row.needsYou && "bg-selected hover:bg-selected")}
      data-needs-you={row.needsYou ? "true" : undefined}
      data-run-id={run.id}
      data-status={run.status}
      data-testid="loop-run-row"
    >
      <TableCell className="w-full min-w-64 max-w-0 py-2.5">
        <span className="flex min-w-0 flex-col gap-0.5">
          <span className="flex min-w-0 items-center gap-2">
            <Link
              className="min-w-0 truncate text-ws-name font-medium text-fg-strong underline-offset-3 hover:underline"
              data-testid="loop-run-name"
              params={{ runId: run.id }}
              search={linkSearch}
              title={run.id}
              to="/loop-runs/$runId"
            >
              {run.loop_name}
            </Link>
            {owner ? <ProfileOwnerTag className="shrink-0" owner={owner} /> : null}
          </span>
          {row.summaryLine ? (
            <span className="truncate text-small-body text-subtle" data-testid="loop-run-summary">
              {row.summaryLine}
            </span>
          ) : null}
        </span>
      </TableCell>
      {project ? (
        <TableCell className="max-w-48" data-testid="loop-run-project">
          <WorkspaceBadge identity={project} />
        </TableCell>
      ) : null}
      <TableCell>
        <span
          className="inline-flex items-center gap-1.75 text-small-body text-fg-2"
          data-state={row.statusGlyph}
          data-testid="loop-run-status"
        >
          <StateGlyph state={row.statusGlyph} still={row.statusStill} />
          {row.statusLabel}
        </span>
      </TableCell>
      <TableCell className={META_CELL} data-testid="loop-run-progress">
        {row.progressLabel}
      </TableCell>
      <TableCell className={META_CELL} data-testid="loop-run-started">
        <Time iso={run.created_at} />
      </TableCell>
      <TableCell className={META_CELL} data-testid="loop-run-duration">
        {formatClockDuration(runElapsedSeconds(run, nowMs))}
      </TableCell>
    </TableRow>
  );
}
