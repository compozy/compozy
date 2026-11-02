import { AlertCircle, RefreshCw } from "@compozy/ui/icons";

import { Button, Empty, ListingPage } from "@compozy/ui";

import { agentFleetListStatus, isAgentsErrorEmpty } from "./agents-catalog-view";
import { useAgentsFleetPage } from "./use-agents-catalog";
import { useAgentsCatalogTopbar } from "./use-agents-catalog-topbar";
import { AgentTeamPage, type AgentsFleetSearch } from "@/systems/agent";

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

  return (
    <ListingPage data-testid="agent-fleet-page">
      {isAgentsErrorEmpty(page) ? (
        <AgentFleetErrorState onRetry={page.retryAgents} />
      ) : (
        <AgentTeamPage
          actions={page.actions}
          members={page.members}
          onClearFilters={page.clearFilters}
          onCreate={page.openCreate}
          status={agentFleetListStatus(page)}
          unfiltered={page.unfiltered}
          view={page.view}
        />
      )}
    </ListingPage>
  );
}
