import { Plus, Users2 } from "lucide-react";

import { Button, useTopbarSlot } from "@compozy/ui";

import { agentsFleetHeadCount, type AgentsFleetPage } from "./agents-catalog-view";
import { AgentFleetToolbar } from "@/systems/agent";

/** Publishes the fleet count, create action, and search toolbar into the window topbar. */
export function useAgentsCatalogTopbar(page: AgentsFleetPage) {
  const noWorkspace = page.workspaceId === "";
  useTopbarSlot({
    glyph: <Users2 />,
    count: agentsFleetHeadCount(page),
    actions:
      page.isFirstRunEmpty || noWorkspace ? undefined : (
        <div className="flex items-center gap-2" data-testid="agents-topbar-actions">
          <Button
            data-testid="agents-topbar-create"
            onClick={page.openCreate}
            size="sm"
            type="button"
            variant="secondary"
          >
            <Plus aria-hidden="true" />
            New agent
          </Button>
        </div>
      ),
    toolbar: noWorkspace ? undefined : (
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
}
