import type { PillGroupItem } from "@compozy/ui";

import type { AgentSessionFilter } from "../lib/agent-detail-search";

/** Filter pills carry the catalog counts; counts stay hidden until the aggregate is exact. */
export function sessionFilterItems(
  counts: {
    total: number;
    active: number;
    failed: number | null;
  } | null
): PillGroupItem<AgentSessionFilter>[] {
  return [
    {
      value: "all",
      label: "All",
      ...(counts ? { badge: counts.total } : {}),
      testId: "agent-sessions-filter-all",
    },
    {
      value: "active",
      label: "Active",
      ...(counts ? { badge: counts.active } : {}),
      testId: "agent-sessions-filter-active",
    },
    {
      value: "failed",
      label: "Failed",
      ...(counts && counts.failed !== null ? { badge: counts.failed } : {}),
      testId: "agent-sessions-filter-failed",
    },
    { value: "done", label: "Done", testId: "agent-sessions-filter-done" },
  ];
}
