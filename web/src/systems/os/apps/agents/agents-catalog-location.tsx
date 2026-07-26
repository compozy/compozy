import { AlertCircle, Plus, RefreshCw, Users2 } from "lucide-react";

import { Button, Empty, ListingPage, useTopbarSlot } from "@agh/ui";

import { AgentFleetList, AgentFleetToolbar, type AgentsFleetSearch } from "@/systems/agent";
import { useAgentsFleetPage } from "./use-agents-catalog";

export function AgentsCatalogLocation({ search }: { search: AgentsFleetSearch }) {
  const page = useAgentsFleetPage(search);

  const agentsErrorEmpty = Boolean(page.agentsError) && page.agents.length === 0 && !page.isLoading;
  const headCount =
    page.isLoading || agentsErrorEmpty || page.isFirstRunEmpty || page.workspaceId === ""
      ? undefined
      : page.fleetTotal;

  useTopbarSlot({
    glyph: <Users2 />,
    count: headCount,
    actions:
      page.isFirstRunEmpty || page.workspaceId === "" ? undefined : (
        <div className="flex items-center gap-2" data-testid="agents-topbar-actions">
          <Button
            data-testid="agents-topbar-create"
            onClick={page.openCreate}
            size="sm"
            type="button"
          >
            <Plus aria-hidden="true" className="size-3" />
            New agent
          </Button>
        </div>
      ),
    toolbar:
      page.workspaceId === "" ? undefined : (
        <AgentFleetToolbar
          categoryOptions={page.categoryOptions}
          draftQuery={page.draftQuery}
          onDraftQueryChange={page.setDraftQuery}
          onFiltersChange={page.setFilters}
          onViewChange={page.setView}
          search={page.search}
          searchInputRef={page.searchInputRef}
          showFacets={page.showFacets}
          showViewToggle={page.showViewToggle}
          view={page.view}
        />
      ),
  });

  if (page.workspaceId === "") {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="agents-no-workspace"
      >
        <Empty
          description="Select a workspace to browse its agents."
          icon={Users2}
          title="No workspace selected"
        />
      </div>
    );
  }

  return (
    <ListingPage data-testid="agent-fleet-page">
      {agentsErrorEmpty ? (
        <div
          className="flex min-h-0 flex-1 items-center justify-center py-10"
          data-testid="agent-fleet-error"
        >
          <Empty
            action={
              <Button
                data-testid="agent-fleet-error-retry"
                onClick={page.retryAgents}
                size="sm"
                type="button"
                variant="ghost"
              >
                <RefreshCw aria-hidden="true" className="size-3" />
                Retry
              </Button>
            }
            description="The agents request failed. Check the daemon connection and try again."
            icon={AlertCircle}
            title="Couldn't load agents"
          />
        </div>
      ) : (
        <AgentFleetList
          status={
            page.isLoading
              ? "loading"
              : page.isFirstRunEmpty
                ? "first-run-empty"
                : page.isFilteredEmpty
                  ? "filtered-empty"
                  : "ready"
          }
          newSessionStatus={page.newSessionDisabled ? "disabled" : "enabled"}
          onClearFilters={page.clearFilters}
          onCreateAgent={page.openCreate}
          onNewSession={page.openNewSession}
          paginationStatus={page.isLoadingMore ? "loading" : page.hasMore ? "available" : undefined}
          onLoadMore={page.loadMore}
          rows={page.rows}
          sessionDataStatus={page.sessionsPartial ? "partial" : "available"}
          view={page.view}
        />
      )}
    </ListingPage>
  );
}
