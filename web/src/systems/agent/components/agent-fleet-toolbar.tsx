import { ListFilter } from "lucide-react";
import type { RefObject } from "react";

import {
  Button,
  FiltersWithSearch,
  ListingToolbar,
  SearchInput,
  type Filter,
  type ListingViewMode,
} from "@agh/ui";

import {
  agentFleetFiltersToChips,
  agentFleetChipsToFilters,
  buildAgentFleetFilterFields,
} from "../lib/agent-fleet-filters";
import type { AgentsFleetSearch } from "../lib/agent-fleet-search";

export interface AgentFleetToolbarProps {
  search: AgentsFleetSearch;
  draftQuery: string;
  categoryOptions: readonly string[];
  searchInputRef: RefObject<HTMLInputElement | null>;
  showFacets: boolean;
  showViewToggle?: boolean;
  view: ListingViewMode;
  onDraftQueryChange: (next: string) => void;
  onFiltersChange: (next: Pick<AgentsFleetSearch, "category" | "status">) => void;
  onViewChange: (next: ListingViewMode) => void;
}

function AgentFleetToolbar({
  search,
  draftQuery,
  categoryOptions,
  searchInputRef,
  showFacets,
  showViewToggle = true,
  view,
  onDraftQueryChange,
  onFiltersChange,
  onViewChange,
}: AgentFleetToolbarProps) {
  const fields = buildAgentFleetFilterFields(categoryOptions);
  const chips = agentFleetFiltersToChips(search);

  const handleFiltersChange = (next: Filter<string>[]) => {
    onFiltersChange(agentFleetChipsToFilters(next));
  };

  return (
    <ListingToolbar data-testid="agent-fleet-toolbar">
      <ListingToolbar.Leading>
        <SearchInput
          ref={searchInputRef}
          aria-label="Search agents"
          data-testid="agent-fleet-search"
          kbd="/"
          onChange={onDraftQueryChange}
          placeholder="Search agents"
          value={draftQuery}
        />
        {showFacets ? (
          <ListingToolbar.Filters>
            <FiltersWithSearch<string>
              allowMultiple={false}
              data-testid="agent-fleet-filters"
              fields={fields}
              filters={chips}
              onChange={handleFiltersChange}
              size="sm"
              trigger={
                <Button
                  aria-label="Add filter"
                  data-testid="agent-fleet-filters-add"
                  size="sm"
                  type="button"
                  variant="ghost"
                >
                  <ListFilter aria-hidden="true" className="size-3" />
                  Filter
                </Button>
              }
            />
          </ListingToolbar.Filters>
        ) : null}
      </ListingToolbar.Leading>
      {showViewToggle ? (
        <ListingToolbar.Trailing>
          <ListingToolbar.ViewToggle onChange={onViewChange} value={view} />
        </ListingToolbar.Trailing>
      ) : null}
    </ListingToolbar>
  );
}

export { AgentFleetToolbar };
