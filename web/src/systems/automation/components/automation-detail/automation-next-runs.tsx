import type { ComponentProps } from "react";

import { cn } from "@compozy/ui";

import type { AutomationNextRun } from "../../lib/automation-detail";

/** A single upcoming fire time: index chip + relative label + absolute UTC stamp. */
function NextRunRow({ run }: { run: AutomationNextRun }) {
  return (
    <li className="flex items-center gap-3">
      <span
        className={cn(
          "grid size-[18px] flex-none place-items-center rounded-full font-mono text-badge font-semibold",
          run.isFirst ? "bg-accent-tint-strong text-accent-strong" : "bg-surface-2 text-subtle"
        )}
      >
        {run.index}
      </span>
      <span
        className={cn(
          "flex-1 text-small-body font-medium",
          run.isFirst ? "text-fg-strong" : "text-fg"
        )}
      >
        {run.relative}
        {run.oneTime ? " · one-time" : ""}
      </span>
      <span className="font-mono text-form-hint text-subtle tabular-nums">{run.absolute}</span>
    </li>
  );
}

interface AutomationNextRunsProps extends Omit<ComponentProps<"ol">, "children"> {
  runs: readonly AutomationNextRun[];
}

/**
 * Upcoming fire times of a schedule, shared by the detail Starts row and the
 * editor preview. Renders nothing for an empty list; the caller owns the
 * reason (Off, past, counted from turn-on).
 */
export function AutomationNextRuns({ runs, className, ...props }: AutomationNextRunsProps) {
  if (runs.length === 0) return null;
  return (
    <ol
      aria-label="Next runs"
      className={cn("flex flex-col gap-1.5", className)}
      data-testid="automation-next-runs"
      {...props}
    >
      {runs.map(run => (
        <NextRunRow key={run.index} run={run} />
      ))}
    </ol>
  );
}
