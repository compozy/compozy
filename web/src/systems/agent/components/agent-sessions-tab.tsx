import { PillGroup, Skeleton, type PillGroupItem } from "@agh/ui";

import type { SessionPayload } from "@/systems/session";

import { filterAgentSessionsByStatus, type AgentSessionFilter } from "../lib/agent-detail-search";
import { AgentSessionsList } from "./agent-sessions-list";
import { AgentStatsGrid } from "./agent-stats-grid";
import { formatAgentRuntimeDuration } from "../lib/format-agent-runtime-duration";

const FILTER_ITEMS: PillGroupItem<AgentSessionFilter>[] = [
  { value: "all", label: "All", testId: "agent-sessions-filter-all" },
  { value: "active", label: "Active", testId: "agent-sessions-filter-active" },
  { value: "failed", label: "Failed", testId: "agent-sessions-filter-failed" },
  { value: "done", label: "Done", testId: "agent-sessions-filter-done" },
];

export interface AgentSessionsTabProps {
  agentName: string;
  sessions: SessionPayload[];
  total: number;
  active: number;
  failed: number | null;
  runtimeSeconds?: number | null;
  metricsUnavailable?: boolean;
  metricsLoading?: boolean;
  lastActivityAt: string | null;
  /** Row-list status only — independent of catalog metrics. */
  status: "loading" | "error" | "ready";
  paginationStatus?: "available" | "loading";
  onLoadMore: () => void;
  filter: AgentSessionFilter;
  onFilterChange: (filter: AgentSessionFilter) => void;
  onNewSession: () => void;
  onClearFilter: () => void;
}

export function AgentSessionsTab({
  agentName,
  sessions,
  total,
  active,
  failed,
  runtimeSeconds = null,
  metricsUnavailable = false,
  metricsLoading = false,
  lastActivityAt,
  status,
  paginationStatus,
  onLoadMore,
  filter,
  onFilterChange,
  onNewSession,
  onClearFilter,
}: AgentSessionsTabProps) {
  const filtered = filterAgentSessionsByStatus(sessions, filter);
  const emptyTitle = filter === "all" ? "No sessions for this agent" : `No ${filter} sessions`;
  const emptyDescription =
    filter === "all"
      ? "Start a session to see it here."
      : "Try another filter or show all sessions.";
  const emptyAction =
    filter === "all" ? (
      <button
        type="button"
        className="text-small-body text-accent hover:underline"
        onClick={onNewSession}
        data-testid="agent-sessions-empty-new"
      >
        New session
      </button>
    ) : (
      <button
        type="button"
        className="text-small-body text-accent hover:underline"
        onClick={onClearFilter}
        data-testid="agent-sessions-show-all"
      >
        Show all
      </button>
    );

  return (
    <div className="flex flex-col gap-4" data-testid="agent-sessions-tab">
      {metricsLoading ? (
        <div
          className="grid grid-cols-2 gap-3 md:grid-cols-4"
          data-testid="agent-sessions-metrics-loading"
        >
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-20 rounded-md" />
          ))}
        </div>
      ) : (
        <AgentStatsGrid
          variant="sessions"
          active={active}
          runtimeLabel={runtimeSeconds === null ? null : formatAgentRuntimeDuration(runtimeSeconds)}
          failed={failed}
          sessionsTotal={total}
          lastActivityAt={lastActivityAt}
          metricsAvailable={!metricsUnavailable}
        />
      )}
      <PillGroup
        aria-label="Session filter"
        data-testid="agent-sessions-filter"
        items={FILTER_ITEMS}
        onChange={onFilterChange}
        size="md"
        value={filter}
      />
      <AgentSessionsList
        agentName={agentName}
        sessions={filtered}
        status={status}
        emptyTitle={emptyTitle}
        emptyDescription={emptyDescription}
        emptyAction={emptyAction}
        paginationStatus={paginationStatus}
        onLoadMore={onLoadMore}
      />
    </div>
  );
}
