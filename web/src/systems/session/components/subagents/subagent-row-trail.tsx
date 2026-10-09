import { cn } from "@compozy/ui";

import { SubagentElapsed } from "./subagent-elapsed";
import { SUBAGENT_STATUS_WORD, subagentElapsedClock } from "./subagent-format";
import type { SubagentView } from "./types";

/**
 * Compact row trailing slot (chip preview, inspector roster): elapsed while
 * running or once completed, otherwise the status word — `Failed` in danger,
 * `Waiting for you` in full ink.
 */
export function SubagentRowTrail({
  subagent,
  stale = false,
  className,
}: {
  subagent: SubagentView;
  stale?: boolean;
  className?: string;
}) {
  const clock = subagentElapsedClock(subagent, { stale });
  if ((subagent.status === "running" || subagent.status === "completed") && clock.kind !== "none") {
    return <SubagentElapsed clock={clock} className={className} />;
  }
  return (
    <span
      className={cn(
        "shrink-0 text-transcript-caption whitespace-nowrap",
        subagent.status === "failed"
          ? "text-danger"
          : subagent.status === "waiting"
            ? "text-fg"
            : "text-subtle",
        className
      )}
    >
      {SUBAGENT_STATUS_WORD[subagent.status]}
    </span>
  );
}
