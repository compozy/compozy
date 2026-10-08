import { useId, useState } from "react";
import type { ComponentProps } from "react";
import { ChevronRight } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { Button, Skeleton, StateGlyph, cn } from "@compozy/ui";

import {
  type LoopChildRunRead as LoopChildRunReadState,
  useLoopChildRun,
  useLoopChildRunInputs,
} from "../../hooks/use-loop-child-run";
import { type LoopChildRunInputs, childRunInputLabels } from "../../lib/loop-run-child-inputs";
import { type LoopStepChildRun, childRunsToggleLabel } from "../../lib/loop-run-child-runs";
import { humanizeLoopNodeId } from "../../lib/loop-node-labels";
import { LoopStatusMark } from "../loop-status-mark";
import { useLoopRunChildRead } from "../../hooks/use-loop-run-child-read";

/** Rows read before the list asks; each open row keeps three reads polling. */
const CHILD_RUNS_VISIBLE_LIMIT = 8;
/**
 * How deep a nested loop opens in place. Past it, the child's own page is one
 * click away; a tree drawn any deeper stops fitting the step it hangs from.
 */
const CHILD_RUNS_MAX_DEPTH = 3;

function LoopRunChildRunStatus({ summary, isLoading }: LoopChildRunReadState) {
  if (summary) {
    return <LoopStatusMark data-testid="loop-run-child-run-status" status={summary.status} />;
  }
  return isLoading ? <Skeleton className="h-3.5 w-16" /> : null;
}

/** Where the child is: its current step, why it sits there and for how long. */
function LoopRunChildRunStep({ summary, isLoading, isError }: LoopChildRunReadState) {
  if (isError) return "Couldn't read this child run. Open it to see where it is.";
  const step = summary?.currentStep;
  if (step) {
    return (
      <>
        <StateGlyph className="shrink-0" size="sm" state={step.chip.glyph} />
        <span className="min-w-0 truncate">
          At <span className="text-fg-2">{step.name}</span>
          {step.detail}
        </span>
        {summary.onStepLabel ? (
          <span
            className="shrink-0 tabular-nums text-subtle"
            data-testid="loop-run-child-run-on-step"
          >
            {`· ${summary.onStepLabel}`}
          </span>
        ) : null}
      </>
    );
  }
  return isLoading ? <Skeleton className="h-3 w-32" /> : null;
}

interface LoopRunChildRunRowProps {
  child: LoopStepChildRun;
  inputs: LoopChildRunInputs | undefined;
  depth: number;
}

function LoopRunChildRunRow({ child, inputs, depth }: LoopRunChildRunRowProps) {
  const { workspaceId, nowMs } = useLoopRunChildRead();
  const read = useLoopChildRun(workspaceId, child.runId, nowMs);
  const grandchildren = read.summary?.childRuns ?? [];
  return (
    <li
      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-0.5 px-3 py-2"
      data-child-run-id={child.runId}
      data-depth={depth}
      data-testid="loop-run-child-run"
    >
      <span className="flex min-w-0 items-baseline gap-1.5">
        {child.slotLabel ? (
          <span className="shrink-0 text-small-body text-subtle">
            {humanizeLoopNodeId(child.slotLabel)}
          </span>
        ) : null}
        <Link
          className="shrink-0 text-small-body font-medium text-fg-strong underline-offset-3 hover:underline focus-visible:underline"
          data-testid="loop-run-child-run-link"
          params={{ runId: child.runId }}
          title={child.runId}
          to="/loop-runs/$runId"
        >
          {read.summary?.loopName ?? child.runId}
        </Link>
        {inputs ? (
          <span
            className="min-w-0 truncate text-form-hint text-muted"
            data-testid="loop-run-child-run-inputs"
            title={inputs.title}
          >
            {inputs.label}
          </span>
        ) : null}
      </span>
      <span className="justify-self-end">
        <LoopRunChildRunStatus {...read} />
      </span>
      <span
        className="flex min-w-0 items-center gap-1.5 text-form-hint text-muted"
        data-testid="loop-run-child-run-step"
      >
        <LoopRunChildRunStep {...read} />
      </span>
      <span
        className="justify-self-end whitespace-nowrap font-mono text-mono-id tabular-nums text-subtle"
        data-testid="loop-run-child-run-meta"
      >
        {read.summary?.metaLabel}
      </span>
      {grandchildren.length > 0 && depth < CHILD_RUNS_MAX_DEPTH ? (
        <LoopRunChildRuns
          className="col-span-2 border-l border-line-soft pl-3"
          childRuns={grandchildren}
          depth={depth + 1}
        />
      ) : null}
    </li>
  );
}

interface LoopRunChildRunListProps {
  childRuns: readonly LoopStepChildRun[];
  depth: number;
  id: string;
}

function LoopRunChildRunList({ childRuns, depth, id }: LoopRunChildRunListProps) {
  const { workspaceId } = useLoopRunChildRead();
  const [showAll, setShowAll] = useState(false);
  const visible = showAll ? childRuns : childRuns.slice(0, CHILD_RUNS_VISIBLE_LIMIT);
  const hidden = childRuns.length - visible.length;
  const inputs = childRunInputLabels(
    useLoopChildRunInputs(
      workspaceId,
      visible.map(child => child.runId)
    )
  );
  return (
    <div className="mt-1" id={id}>
      <ul
        className={cn(
          "flex flex-col divide-y divide-line-soft",
          // The first level is a recessed well under its step; a nested level
          // hangs from its parent row by the rule alone — never a card in a card.
          depth === 0 && "overflow-hidden rounded-md bg-sunken"
        )}
      >
        {visible.map(child => (
          <LoopRunChildRunRow
            child={child}
            depth={depth}
            inputs={inputs.get(child.runId)}
            key={child.key}
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
  );
}

export interface LoopRunChildRunsProps extends Omit<ComponentProps<"div">, "children"> {
  childRuns: readonly LoopStepChildRun[];
  /** Open on mount — the node panel, which exists to show one step in full. */
  defaultOpen?: boolean;
  /** Nesting level; a child run's own children open one level down. */
  depth?: number;
}

/**
 * The loop runs a step started, one disclosure below the step.
 *
 * Closed by default: the step's chip already says it is waiting on a child, and
 * the default read stays calm. Opening it reads each child where it lives —
 * the inputs that set it apart from its siblings, its status, the step it is on,
 * why it is parked there and for how long, and how far through it is — so a
 * stuck child shows up without leaving the parent. A child that started loops
 * of its own opens them the same way, a level down.
 */
export function LoopRunChildRuns({
  childRuns,
  defaultOpen = false,
  depth = 0,
  className,
  ...props
}: LoopRunChildRunsProps) {
  const [open, setOpen] = useState(defaultOpen);
  const listId = useId();
  if (childRuns.length === 0) return null;
  return (
    <div className={cn("mt-1", className)} data-testid="loop-run-child-runs" {...props}>
      <Button
        aria-controls={open ? listId : undefined}
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
      {open ? <LoopRunChildRunList childRuns={childRuns} depth={depth} id={listId} /> : null}
    </div>
  );
}
