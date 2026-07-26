import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";

import { formatRelativeTime } from "@agh/ui";

import { formatRunInputs, loopBudgetBar, runGenerationLabel } from "../../lib/loop-runs-view";
import type { LoopRun } from "../../types";
import { LoopStatusPill } from "../loop-status-pill";
import { LoopBudgetMiniBar } from "./loop-budget-mini-bar";

interface LoopRunRowProps {
  run: LoopRun;
}

export const LOOP_RUNS_ROW_GRID =
  "grid-cols-[128px_minmax(0,1.4fr)_minmax(0,1fr)_84px_112px_128px_16px]";

/** Trigger line for a run (`schedule`, `cli`). */
function triggerLabel(run: LoopRun): string {
  if (run.started_origin_kind === "session") {
    return run.started_origin_ref ? `session · ${run.started_origin_ref}` : "session";
  }
  return run.started_origin_kind || run.started_by_kind || "manual";
}

/**
 * One workspace-wide runs table row: outcome pill, loop + id/trigger, resolved
 * inputs, generations vs cap, when it started, budget mini-bar, and a chevron to
 * the run detail.
 */
export function LoopRunRow({ run }: LoopRunRowProps) {
  const inputs = formatRunInputs(run.inputs);
  return (
    <Link
      to="/loop-runs/$runId"
      params={{ runId: run.id }}
      className={`grid ${LOOP_RUNS_ROW_GRID} items-center gap-3.5 border-b border-line-soft px-4 py-3 transition-colors last:border-b-0 hover:bg-row-hover max-[1140px]:grid-cols-[128px_minmax(0,1fr)_auto] max-[1140px]:gap-3`}
      data-testid="loop-run-row"
      data-status={run.status}
    >
      <LoopStatusPill status={run.status} />
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="truncate text-ws-name font-medium text-fg-strong">{run.loop_name}</span>
        <span className="font-mono text-mono-id text-faint">
          {run.id} · {triggerLabel(run)}
        </span>
      </span>
      <span className="truncate text-small-body text-muted max-[1140px]:hidden">
        {inputs || "—"}
      </span>
      <span className="font-mono text-mono-id tabular-nums text-muted max-[1140px]:hidden">
        {runGenerationLabel(run)}
      </span>
      <span className="text-xs tabular-nums text-muted max-[1140px]:text-right">
        {formatRelativeTime(run.created_at)}
      </span>
      <span className="max-[1140px]:hidden">
        <LoopBudgetMiniBar bar={loopBudgetBar(run)} />
      </span>
      <span className="justify-self-end text-faint max-[1140px]:hidden">
        <ChevronRight aria-hidden="true" className="size-3.5" />
      </span>
    </Link>
  );
}
