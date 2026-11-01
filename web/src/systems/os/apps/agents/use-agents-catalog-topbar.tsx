import { Plus, Users2 } from "@compozy/ui/icons";

import { Button, useTopbarSlot } from "@compozy/ui";

import { agentsFleetHeadCount, type AgentsFleetPage } from "./agents-catalog-view";
import { AgentFleetToolbar } from "@/systems/agent";

/** Publishes the fleet count, create action, and search toolbar into the window topbar. */
export function useAgentsCatalogTopbar(page: AgentsFleetPage) {
  useTopbarSlot({
    glyph: <Users2 />,
    count: agentsFleetHeadCount(page),
    actions: page.isFirstRunEmpty ? undefined : (
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
    toolbar: (
      <AgentFleetToolbar
        categoryOptions={page.categoryOptions}
        definitionsFor={page.definitionsFor}
        draftQuery={page.draftQuery}
        onDraftQueryChange={page.setDraftQuery}
        onFiltersChange={page.setFilters}
        onProjectsChange={page.setProjectFilter}
        onViewChange={page.setView}
        projectField={page.projectField}
        projects={page.projectFilter}
        search={page.search}
        searchInputRef={page.searchInputRef}
        showFacets={page.showFacets}
        showViewToggle={page.showViewToggle}
        view={page.view}
      />
    ),
  });
}
