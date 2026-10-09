import type { ComponentProps } from "react";

import { cn, formatDuration } from "@compozy/ui";

import type { SubagentElapsedClock } from "./subagent-format";
import { useSubagentElapsedTicker } from "./use-subagent-elapsed-ticker";

export interface SubagentElapsedProps extends Omit<ComponentProps<"span">, "children"> {
  clock: SubagentElapsedClock;
}

const ELAPSED_CLASS = "shrink-0 font-mono text-mono-id whitespace-nowrap tabular-nums";

function TickingElapsed({
  startMs,
  className,
  ...props
}: Omit<ComponentProps<"span">, "children"> & { startMs: number }) {
  const ref = useSubagentElapsedTicker(startMs);
  return (
    <span
      ref={ref}
      data-slot="subagent-elapsed"
      data-live="true"
      className={cn(ELAPSED_CLASS, "text-subtle", className)}
      {...props}
    />
  );
}

/**
 * Compact elapsed from daemon timestamps: ticking on the shared 1 s ticker
 * while live, frozen once settled (or while stale), absent before the start.
 */
export function SubagentElapsed({ clock, className, ...props }: SubagentElapsedProps) {
  if (clock.kind === "none") return null;
  if (clock.kind === "ticking") {
    return <TickingElapsed startMs={clock.startMs} className={className} {...props} />;
  }
  return (
    <span
      data-slot="subagent-elapsed"
      className={cn(ELAPSED_CLASS, "text-faint", className)}
      {...props}
    >
      {formatDuration(clock.ms, { padded: true })}
    </span>
  );
}
