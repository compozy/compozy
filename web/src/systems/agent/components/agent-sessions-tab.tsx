import { Button, PillGroup } from "@compozy/ui";

import { filterAgentSessionsByStatus, type AgentSessionFilter } from "../lib/agent-detail-search";
import { sessionFilterItems } from "./agent-sessions-filter-items";
import { AgentSessionsList } from "./agent-sessions-list";
import type { SessionLifecycleActionHandlers, SessionPayload } from "@/systems/session";

const EMPTY_ARCHIVED_SESSIONS: SessionPayload[] = [];

export interface AgentSessionsTabProps {
  agentName: string;
  sessions: SessionPayload[];
  archivedSessions?: SessionPayload[];
  archivedTotal?: number;
  total: number;
  active: number;
  failed: number | null;
  metricsUnavailable?: boolean;
  metricsLoading?: boolean;
  /** Row-list status only — independent of catalog metrics. */
  status: "loading" | "error" | "ready";
  paginationStatus?: "available" | "loading";
  archivedPaginationStatus?: "available" | "loading";
  onLoadMore: () => void;
  onLoadMoreArchived?: () => void;
  sessionActions?: SessionLifecycleActionHandlers;
  filter: AgentSessionFilter;
  onFilterChange: (filter: AgentSessionFilter) => void;
  onNewSession: () => void;
  onClearFilter: () => void;
  onRetry?: () => void;
}

export function AgentSessionsTab({
  agentName,
  sessions,
  archivedSessions = EMPTY_ARCHIVED_SESSIONS,
  archivedTotal,
  total,
  active,
  failed,
  metricsUnavailable = false,
  metricsLoading = false,
  status,
  paginationStatus,
  archivedPaginationStatus,
  onLoadMore,
  onLoadMoreArchived,
  sessionActions,
  filter,
  onFilterChange,
  onNewSession,
  onClearFilter,
  onRetry,
}: AgentSessionsTabProps) {
  const filtered = filterAgentSessionsByStatus(sessions, filter);
  const metricsReady = !metricsLoading && !metricsUnavailable;
  const filterItems = sessionFilterItems(metricsReady ? { total, active, failed } : null);
  const emptyTitle = filter === "all" ? "No sessions for this agent" : `No ${filter} sessions`;
  const emptyDescription =
    filter === "all"
      ? "Start a session to see it here."
      : "Try another filter or show all sessions.";
  const emptyAction =
    filter === "all" ? (
      <Button
        data-testid="agent-sessions-empty-new"
        onClick={onNewSession}
        size="sm"
        type="button"
        variant="secondary"
      >
        New session
      </Button>
    ) : (
      <Button
        data-testid="agent-sessions-show-all"
        onClick={onClearFilter}
        size="sm"
        type="button"
        variant="link"
      >
        Show all
      </Button>
    );

  return (
    <div className="flex flex-col gap-4" data-testid="agent-sessions-tab">
      <PillGroup
        aria-label="Session filter"
        data-testid="agent-sessions-filter"
        items={filterItems}
        onChange={onFilterChange}
        size="md"
        value={filter}
      />
      <AgentSessionsList
        agentName={agentName}
        sessions={filtered}
        archivedSessions={archivedSessions}
        archivedTotal={archivedTotal}
        status={status}
        emptyTitle={emptyTitle}
        emptyDescription={emptyDescription}
        emptyAction={emptyAction}
        paginationStatus={paginationStatus}
        onLoadMore={onLoadMore}
        archivedPaginationStatus={archivedPaginationStatus}
        onLoadMoreArchived={onLoadMoreArchived}
        sessionActions={sessionActions}
        onRetry={onRetry}
      />
    </div>
  );
}
