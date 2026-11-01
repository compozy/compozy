import { ListFilter } from "@compozy/ui/icons";
import type { RefObject } from "react";

import {
  Button,
  FiltersWithSearch,
  ListingToolbar,
  SearchInput,
  type Filter,
  type FilterFieldConfig,
  type ListingViewMode,
} from "@compozy/ui";
import {
  projectFilterChip,
  projectFilterValuesFromChips,
  type ProjectFilterValue,
} from "@/systems/workspace";

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
  /** All projects only: the shared Project filter field, its selection and change. */
  projectField?: FilterFieldConfig<string> | null;
  projects?: readonly ProjectFilterValue[];
  onProjectsChange?: (next: readonly ProjectFilterValue[]) => void;
  /** All profiles: definitions resolve for one profile, so the toolbar names it. */
  definitionsFor?: string | null;
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
  projectField,
  projects = [],
  onProjectsChange,
  definitionsFor,
}: AgentFleetToolbarProps) {
  const fields = buildAgentFleetFilterFields(categoryOptions, projectField);
  const projectChip = projectFilterChip(projects);
  const chips = [...agentFleetFiltersToChips(search), ...(projectChip ? [projectChip] : [])];

  const handleFiltersChange = (next: Filter<string>[]) => {
    onFiltersChange(agentFleetChipsToFilters(next));
    onProjectsChange?.(projectFilterValuesFromChips(next));
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
                  variant="quiet"
                >
                  <ListFilter aria-hidden="true" />
                  Filter
                </Button>
              }
            />
          </ListingToolbar.Filters>
        ) : null}
      </ListingToolbar.Leading>
      {showViewToggle || definitionsFor ? (
        <ListingToolbar.Trailing>
          {definitionsFor ? (
            <span
              className="text-micro text-muted-foreground"
              data-testid="agent-fleet-definitions-for"
            >
              Definitions for {definitionsFor}
            </span>
          ) : null}
          {showViewToggle ? (
            <ListingToolbar.ViewToggle onChange={onViewChange} value={view} />
          ) : null}
        </ListingToolbar.Trailing>
      ) : null}
    </ListingToolbar>
  );
}

export { AgentFleetToolbar };
