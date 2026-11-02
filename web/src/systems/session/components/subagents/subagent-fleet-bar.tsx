import { cn } from "@compozy/ui";

import { isSubagentLive } from "./subagent-format";
import type { SubagentView } from "./types";

function barRank(subagent: SubagentView) {
  if (subagent.status === "failed") return 1;
  if (!isSubagentLive(subagent.status)) return 0;
  return subagent.status === "queued" ? 3 : 2;
}

/**
 * One segment per subagent, filling like progress: finished, failed, working, then the queue.
 * It counts subagents, never steps: nothing reports how far along one subagent is.
 */
export function SubagentFleetBar({ subagents }: { subagents: readonly SubagentView[] }) {
  const ordered = [...subagents].sort((a, b) => barRank(a) - barRank(b));
  return (
    <span aria-hidden="true" className="flex h-1 gap-0.5" data-slot="subagent-fleet-bar">
      {ordered.map(subagent => (
        <span
          key={subagent.id}
          data-rank={barRank(subagent)}
          className={cn(
            "min-w-0 flex-1 rounded-full",
            barRank(subagent) === 0 && "bg-success",
            barRank(subagent) === 1 && "bg-destructive",
            barRank(subagent) === 2 && "animate-pulse bg-success/50 motion-reduce:animate-none",
            barRank(subagent) === 3 && "bg-fg/15"
          )}
        />
      ))}
    </span>
  );
}
