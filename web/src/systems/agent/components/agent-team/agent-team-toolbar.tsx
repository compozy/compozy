import { ListFilter } from "@compozy/ui/icons";
import type { RefObject } from "react";

import {
  Button,
  FiltersWithSearch,
  ListingToolbar,
  PillGroup,
  SearchInput,
  type Filter,
  type FilterFieldsConfig,
  type ListingViewMode,
} from "@compozy/ui";

import type { AgentTeamFocus } from "../../lib/agent-team";

export interface AgentTeamToolbarProps {
  draftQuery: string;
  searchInputRef: RefObject<HTMLInputElement | null>;
  fields: FilterFieldsConfig<string>;
  chips: Filter<string>[];
  focus: AgentTeamFocus;
  focusItems: readonly { value: AgentTeamFocus; label: string; badge?: number }[];
  view: ListingViewMode;
  showFacets: boolean;
  /** All profiles: definitions resolve for one profile, so the toolbar names it. */
  definitionsFor?: string | null;
  onDraftQueryChange: (next: string) => void;
  onChipsChange: (next: Filter<string>[]) => void;
  onFocusChange: (next: AgentTeamFocus) => void;
  onViewChange: (next: ListingViewMode) => void;
}

export function AgentTeamToolbar({
  draftQuery,
  searchInputRef,
  fields,
  chips,
  focus,
  focusItems,
  view,
  showFacets,
  definitionsFor,
  onDraftQueryChange,
  onChipsChange,
  onFocusChange,
  onViewChange,
}: AgentTeamToolbarProps) {
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
              onChange={onChipsChange}
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
      <ListingToolbar.Trailing>
        {definitionsFor ? (
          <span
            className="text-micro text-muted-foreground"
            data-testid="agent-fleet-definitions-for"
          >
            Definitions for {definitionsFor}
          </span>
        ) : null}
        {focusItems.length > 1 ? (
          <PillGroup items={focusItems} onChange={onFocusChange} size="sm" value={focus} />
        ) : null}
        <ListingToolbar.ViewToggle onChange={onViewChange} value={view} />
      </ListingToolbar.Trailing>
    </ListingToolbar>
  );
}
