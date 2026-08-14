import { PillDot } from "@compozy/ui";

export interface WorktreeAggregateProps {
  runningAgents: number;
}

export function WorktreeAggregate({ runningAgents }: WorktreeAggregateProps) {
  if (runningAgents <= 0) return null;

  return (
    <span
      data-slot="worktree-aggregate"
      className="inline-flex shrink-0 items-center gap-1 font-mono text-micro text-faint"
    >
      <PillDot tone="accent" pulse size="sm" />
      {runningAgents} active
    </span>
  );
}
