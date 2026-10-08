import { useId, useState } from "react";
import type { ComponentProps } from "react";
import { ChevronRight } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { Button, Skeleton, StateGlyph, cn } from "@compozy/ui";

import { useLoopChildRun } from "../../hooks/use-loop-child-run";
import { type LoopStepChildRun, childRunsToggleLabel } from "../../lib/loop-run-child-runs";
import { humanizeLoopNodeId } from "../../lib/loop-node-labels";
import { loopParkReason } from "../../lib/loop-run-state-copy";
import { formatClockDuration } from "../../lib/loop-run-usage";
import { LoopStatusMark } from "../loop-status-mark";

/** Rows read before the list asks; each open row keeps two reads polling. */
const CHILD_RUNS_VISIBLE_LIMIT = 8;

interface LoopRunChildRunRowProps {
  child: LoopStepChildRun;
  workspaceId: string;
  nowMs: number;
}

function LoopRunChildRunRow({ child, workspaceId, nowMs }: LoopRunChildRunRowProps) {
  const { summary, isLoading, isError } = useLoopChildRun(workspaceId, child.runId, nowMs);
  const slot = child.slotLabel ? humanizeLoopNodeId(child.slotLabel) : null;
  const step = summary?.currentStep ?? null;
  const parkReason = step ? loopParkReason(step.chip.state) : null;
  const meta = [summary?.progressLabel, summary ? formatClockDuration(summary.elapsedSeconds) : ""]
    .filter(Boolean)
    .join(" · ");
  return (
    <li
      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-0.5 px-3 py-2"
      data-child-run-id={child.runId}
      data-testid="loop-run-child-run"
    >
      <span className="flex min-w-0 items-baseline gap-1.5">
        {slot ? <span className="shrink-0 text-small-body text-subtle">{slot}</span> : null}
        <Link
          className="min-w-0 truncate text-small-body font-medium text-fg-strong underline-offset-3 hover:underline focus-visible:underline"
          data-testid="loop-run-child-run-link"
          params={{ runId: child.runId }}
          title={child.runId}
          to="/loop-runs/$runId"
        >
          {summary?.loopName ?? child.runId}
        </Link>
      </span>
      <span className="justify-self-end">
        {summary ? (
          <LoopStatusMark data-testid="loop-run-child-run-status" status={summary.status} />
        ) : isLoading ? (
          <Skeleton className="h-3.5 w-16" />
        ) : null}
      </span>
      <span
        className="flex min-w-0 items-center gap-1.5 text-form-hint text-muted"
        data-testid="loop-run-child-run-step"
      >
        {isError ? (
          "Couldn't read this child run. Open it to see where it is."
        ) : step ? (
          <>
            <StateGlyph className="shrink-0" size="sm" state={step.chip.glyph} />
            <span className="min-w-0 truncate">
              At <span className="text-fg-2">{humanizeLoopNodeId(step.nodeId)}</span>
              {parkReason ? ` — ${parkReason}` : null}
              {step.alsoActive > 0 ? ` · ${step.alsoActive} more active` : null}
            </span>
          </>
        ) : isLoading ? (
          <Skeleton className="h-3 w-32" />
        ) : null}
      </span>
      <span
        className="justify-self-end whitespace-nowrap font-mono text-mono-id tabular-nums text-subtle"
        data-testid="loop-run-child-run-meta"
      >
        {meta}
      </span>
    </li>
  );
}

export interface LoopRunChildRunsProps extends Omit<ComponentProps<"div">, "children"> {
  childRuns: readonly LoopStepChildRun[];
  workspaceId: string;
  /** The page clock, so a live child's elapsed time ticks with the parent's. */
  nowMs: number;
}

/**
 * The loop runs a step started, one disclosure below the step.
 *
 * Closed by default: the step's chip already says it is waiting on a child, and
 * the default read stays calm. Opening it reads each child where it lives —
 * status, the step it is on and why it is parked there, how far through it is
 * and for how long — so a stuck child shows up without leaving the parent.
 */
export function LoopRunChildRuns({
  childRuns,
  workspaceId,
  nowMs,
  className,
  ...props
}: LoopRunChildRunsProps) {
  const [open, setOpen] = useState(false);
  const [showAll, setShowAll] = useState(false);
  const listId = useId();
  if (childRuns.length === 0) return null;
  const visible = showAll ? childRuns : childRuns.slice(0, CHILD_RUNS_VISIBLE_LIMIT);
  const hidden = childRuns.length - visible.length;
  return (
    <div className={cn("mt-1", className)} data-testid="loop-run-child-runs" {...props}>
      <Button
        aria-controls={listId}
        aria-expanded={open}
        className="-ml-2 text-muted hover:text-fg"
        data-testid="loop-run-child-runs-toggle"
        onClick={() => setOpen(value => !value)}
        size="sm"
        type="button"
        variant="ghost"
      >
        <ChevronRight
          aria-hidden="true"
          data-icon="inline-start"
          className={cn(
            "transition-transform duration-150 ease-out motion-reduce:transition-none",
            open && "rotate-90"
          )}
        />
        {childRunsToggleLabel(childRuns.length)}
      </Button>
      {open ? (
        <div className="mt-1" id={listId}>
          <ul className="flex flex-col divide-y divide-line-soft overflow-hidden rounded-md bg-sunken">
            {visible.map(child => (
              <LoopRunChildRunRow
                child={child}
                key={child.key}
                nowMs={nowMs}
                workspaceId={workspaceId}
              />
            ))}
          </ul>
          {hidden > 0 ? (
            <Button
              className="-ml-2.5 mt-1 text-muted hover:text-fg"
              data-testid="loop-run-child-runs-more"
              onClick={() => setShowAll(true)}
              size="sm"
              type="button"
              variant="ghost"
            >
              {`Show ${hidden} more`}
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
