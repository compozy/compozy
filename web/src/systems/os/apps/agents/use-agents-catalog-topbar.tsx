import { Plus, Users2 } from "@compozy/ui/icons";

import { Button, useTopbarSlot } from "@compozy/ui";

import { agentsFleetHeadCount, type AgentsFleetPage } from "./agents-catalog-view";
import { AgentTeamToolbar } from "@/systems/agent";

/** Publishes the fleet count, create action, and search toolbar into the window topbar. */
export function useAgentsCatalogTopbar(page: AgentsFleetPage) {
  useTopbarSlot({
    glyph: <Users2 />,
    count: agentsFleetHeadCount(page),
    actions: page.isFirstRunEmpty ? undefined : (
      <div className="flex items-center gap-2" data-testid="agents-topbar-actions">
        <Button
          data-testid="agents-topbar-create"
          onClick={() => page.openCreate()}
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
      <AgentTeamToolbar
        chips={page.chips}
        definitionsFor={page.definitionsFor}
        draftQuery={page.draftQuery}
        fields={page.fields}
        focus={page.focus}
        focusItems={page.focusItems}
        onChipsChange={page.setChips}
        onDraftQueryChange={page.setDraftQuery}
        onFocusChange={page.setFocus}
        onViewChange={page.setView}
        searchInputRef={page.searchInputRef}
        showFacets={page.showFacets}
        view={page.view}
      />
    ),
  });
}
