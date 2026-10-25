import { PillGroup } from "@compozy/ui";

import type { AgentSessionFilter } from "../lib/agent-detail-search";
import { sessionFilterItems } from "./agent-sessions-filter-items";
import { AgentSessionsSkeleton } from "./agent-sessions-skeleton";

export interface AgentSessionsTabSkeletonProps {
  filter: AgentSessionFilter;
  onFilterChange: (filter: AgentSessionFilter) => void;
  /** Takes the pills out of reach, for a frame shown before the window mounts. */
  inert?: boolean;
}

/** The tab while the agent loads: the real filter pills without counts, over the table skeleton. */
export function AgentSessionsTabSkeleton({
  filter,
  onFilterChange,
  inert = false,
}: AgentSessionsTabSkeletonProps) {
  const pills = (
    <PillGroup
      aria-label="Session filter"
      items={sessionFilterItems(null)}
      onChange={onFilterChange}
      size="md"
      value={filter}
    />
  );
  return (
    <div className="flex flex-col gap-4" data-testid="agent-sessions-tab-skeleton">
      {inert ? (
        <div className="contents" inert>
          {pills}
        </div>
      ) : (
        pills
      )}
      <AgentSessionsSkeleton />
    </div>
  );
}
