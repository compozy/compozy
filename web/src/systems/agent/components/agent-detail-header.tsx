import { Pill, Skeleton } from "@compozy/ui";

import { formatAgentOriginLabel, formatCategoryMetaSegment } from "../lib/agent-fleet-projection";
import type { AgentPayload } from "../types";

export interface AgentDetailHeaderProps {
  agent: AgentPayload;
}

/**
 * Body meta under the unified window head — origin/diagnostics/category only.
 * Title, status, and trail actions publish through `useTopbarSlot` at the location.
 */
export function AgentDetailHeader({ agent }: AgentDetailHeaderProps) {
  const category = formatCategoryMetaSegment(agent.category_path);
  const origin = formatAgentOriginLabel(agent.origin);
  const hasDiagnostics = Array.isArray(agent.diagnostics) && agent.diagnostics.length > 0;

  return (
    <div className="flex flex-wrap items-center gap-1.5 pt-4" data-testid="agent-detail-header">
      {origin ? (
        <Pill size="sm" data-testid="agent-detail-header-origin">
          {origin}
        </Pill>
      ) : null}
      {hasDiagnostics ? (
        <Pill tone="warning" size="sm" data-testid="agent-detail-header-invalid">
          Invalid
        </Pill>
      ) : null}
      {category ? (
        <span
          className="truncate text-small-body text-muted-foreground"
          data-testid="agent-detail-header-meta"
        >
          {category}
        </span>
      ) : null}
    </div>
  );
}

/** The header meta row while the agent loads: one pill-height placeholder where the origin pill goes. */
export function AgentDetailHeaderSkeleton() {
  return (
    <div
      aria-hidden="true"
      className="flex flex-wrap items-center gap-1.5 pt-4"
      data-testid="agent-detail-header-skeleton"
    >
      <Skeleton className="h-pill-sm w-16 rounded-pill" />
    </div>
  );
}
