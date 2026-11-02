import { cn } from "@compozy/ui";

import { subagentDisplay } from "./subagent-format";
import type { SubagentView } from "./types";

/** Ranked from the rows' own display, so a resumed child reads as working in both. */
function barSegment(subagent: SubagentView) {
  const { glyph, settled, tone } = subagentDisplay(subagent);
  if (tone === "destructive") return { rank: 2, tone: "bg-destructive" };
  if (glyph === "stopped") return { rank: 1, tone: "bg-fg/30" };
  if (settled) return { rank: 0, tone: "bg-success" };
  if (glyph === "queued") return { rank: 4, tone: "bg-fg/15" };
  return { rank: 3, tone: "animate-pulse bg-success/50 motion-reduce:animate-none" };
}

/**
 * One segment per subagent, filling like progress: finished, stopped, failed, working, then the queue.
 * It counts subagents, never steps: nothing reports how far along one subagent is.
 */
export function SubagentFleetBar({ subagents }: { subagents: readonly SubagentView[] }) {
  const ordered = [...subagents].sort((a, b) => barSegment(a).rank - barSegment(b).rank);
  return (
    <span aria-hidden="true" className="flex h-1 gap-0.5" data-slot="subagent-fleet-bar">
      {ordered.map(subagent => (
        <span
          key={subagent.id}
          className={cn("min-w-0 flex-1 rounded-full", barSegment(subagent).tone)}
        />
      ))}
    </span>
  );
}
