import { AlertTriangle, Users2 } from "@compozy/ui/icons";

import { Alert, AlertDescription, Button, Empty, Spinner, type ListingViewMode } from "@compozy/ui";

import type { AgentFleetRowModel } from "../lib/agent-fleet-projection";
import { AgentFleetCard } from "./agent-fleet-card";
import { FLEET_CARDS_CLASS, FLEET_ROWS_CLASS } from "./agent-fleet-classes";
import { AgentFleetRow } from "./agent-fleet-row";
import { AgentFleetSkeleton } from "./agent-fleet-skeleton";

export interface AgentFleetListProps {
  rows: readonly AgentFleetRowModel[];
  view: ListingViewMode;
  status: "loading" | "first-run-empty" | "filtered-empty" | "ready";
  sessionDataStatus: "available" | "partial";
  newSessionStatus?: "enabled" | "disabled";
  paginationStatus?: "available" | "loading";
  onClearFilters: () => void;
  onCreateAgent: () => void;
  onNewSession: (agentName: string) => void;
  /** All projects: rows open in place, bound to their project. */
  onOpenAgent?: (row: AgentFleetRowModel) => void;
  onLoadMore: () => void;
}

function AgentFleetList({
  rows,
  view,
  status,
  sessionDataStatus,
  newSessionStatus = "enabled",
  paginationStatus,
  onClearFilters,
  onCreateAgent,
  onNewSession,
  onOpenAgent,
  onLoadMore,
}: AgentFleetListProps) {
  if (status === "loading") return <AgentFleetSkeleton view={view} />;

  if (status === "first-run-empty") {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
        data-testid="agent-fleet-empty"
      >
        <Empty
          action={
            <Button
              data-testid="agent-fleet-empty-create"
              onClick={onCreateAgent}
              size="sm"
              type="button"
            >
              New agent
            </Button>
          }
          description="An agent is an assistant with its own instructions and model. Create one to start working with it."
          icon={Users2}
          title="No agents yet"
        />
      </div>
    );
  }

  if (status === "filtered-empty") {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
        data-testid="agent-fleet-filtered-empty"
      >
        <Empty
          action={
            <Button
              data-testid="agent-fleet-clear-filters"
              onClick={onClearFilters}
              size="sm"
              type="button"
              variant="ghost"
            >
              Clear filters
            </Button>
          }
          description="Try a different search or clear the active filters."
          icon={Users2}
          title="No agents match"
        />
      </div>
    );
  }

  return (
    <div className="min-h-0 flex-1" data-testid="agent-fleet-list">
      {sessionDataStatus === "partial" ? (
        <Alert data-testid="agent-fleet-sessions-notice" role="status" variant="warning">
          <AlertTriangle aria-hidden="true" className="size-4" />
          <AlertDescription>Session status unavailable</AlertDescription>
        </Alert>
      ) : null}
      {view === "cards" ? (
        <div
          className={FLEET_CARDS_CLASS}
          data-slot="agent-fleet-cards"
          data-testid="agent-fleet-card-grid"
        >
          {rows.map(row => (
            <AgentFleetCard
              key={row.key}
              newSessionDisabled={newSessionStatus === "disabled"}
              onNewSession={onNewSession}
              onOpen={onOpenAgent}
              row={row}
            />
          ))}
        </div>
      ) : (
        <div className={FLEET_ROWS_CLASS} data-slot="agent-fleet-rows">
          {rows.map(row => (
            <AgentFleetRow
              key={row.key}
              newSessionDisabled={newSessionStatus === "disabled"}
              onNewSession={onNewSession}
              onOpen={onOpenAgent}
              row={row}
            />
          ))}
        </div>
      )}
      {paginationStatus ? (
        <div className="flex justify-center py-4">
          <Button
            aria-busy={paginationStatus === "loading"}
            data-testid="agent-fleet-load-more"
            disabled={paginationStatus === "loading"}
            onClick={onLoadMore}
            size="sm"
            type="button"
            variant="neutral"
          >
            {paginationStatus === "loading" ? <Spinner aria-hidden="true" /> : null}
            {paginationStatus === "loading" ? "Loading more agents" : "Load more agents"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

export { AgentFleetList };
