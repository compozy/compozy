import { AlertCircle, RefreshCw, Users2 } from "lucide-react";

import { Button, Empty, ListingPage } from "@compozy/ui";

import { agentFleetListStatus, isAgentsErrorEmpty } from "./agents-catalog-view";
import { listPaginationStatus } from "./list-pagination-status";
import { useAgentsFleetPage } from "./use-agents-catalog";
import { useAgentsCatalogTopbar } from "./use-agents-catalog-topbar";
import { AgentFleetList, type AgentsFleetSearch } from "@/systems/agent";

function AgentFleetErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <div
      className="flex min-h-0 flex-1 items-center justify-center py-10"
      data-testid="agent-fleet-error"
    >
      <Empty
        action={
          <Button
            data-testid="agent-fleet-error-retry"
            onClick={onRetry}
            size="sm"
            type="button"
            variant="secondary"
          >
            <RefreshCw aria-hidden="true" />
            Retry
          </Button>
        }
        description="Couldn't load your agents. Check that CompozyOS is running, then try again."
        icon={AlertCircle}
        title="Couldn't load agents"
      />
    </div>
  );
}

export function AgentsCatalogLocation({ search }: { search: AgentsFleetSearch }) {
  const page = useAgentsFleetPage(search);
  useAgentsCatalogTopbar(page);

  if (page.workspaceId === "") {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="agents-no-workspace"
      >
        <Empty
          description="Choose a project to see its agents."
          icon={Users2}
          title="No project selected"
        />
      </div>
    );
  }

  return (
    <ListingPage data-testid="agent-fleet-page">
      {isAgentsErrorEmpty(page) ? (
        <AgentFleetErrorState onRetry={page.retryAgents} />
      ) : (
        <AgentFleetList
          status={agentFleetListStatus(page)}
          newSessionStatus={page.newSessionDisabled ? "disabled" : "enabled"}
          onClearFilters={page.clearFilters}
          onCreateAgent={page.openCreate}
          onNewSession={page.openNewSession}
          paginationStatus={listPaginationStatus(page.isLoadingMore, page.hasMore)}
          onLoadMore={page.loadMore}
          rows={page.rows}
          sessionDataStatus={page.sessionsPartial ? "partial" : "available"}
          view={page.view}
        />
      )}
    </ListingPage>
  );
}
