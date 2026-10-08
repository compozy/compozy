import type { ComponentProps } from "react";

import { cn } from "@compozy/ui";

import type { AutomationNextRun } from "../../lib/automation-detail";

/** A single upcoming fire time: faint index, relative label, absolute UTC stamp. */
function NextRunRow({ run }: { run: AutomationNextRun }) {
  return (
    <li className="flex min-h-7 items-center gap-2.5 border-t border-line-soft text-eyebrow text-muted first:border-t-0">
      <span className="w-4 font-mono text-badge text-faint">{run.index}</span>
      <span className="min-w-21.5 font-medium text-fg">
        {run.relative}
        {run.oneTime ? " · one-time" : ""}
      </span>
      <span className="ml-auto font-mono text-mono-id text-subtle tabular-nums">
        {run.absolute}
      </span>
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
      className={cn("flex flex-col", className)}
      data-testid="automation-next-runs"
      {...props}
    >
      {runs.map(run => (
        <NextRunRow key={run.index} run={run} />
      ))}
    </ol>
  );
}
