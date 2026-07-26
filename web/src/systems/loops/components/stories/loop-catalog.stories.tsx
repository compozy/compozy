import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { ListingToolbar, type ListingViewMode } from "@agh/ui";

import { StorySurface } from "@/storybook/story-layout";

import { LoopCatalog } from "../catalog/loop-catalog";
import { LoopCatalogFilters } from "../catalog/loop-catalog-filters";
import type { LoopCatalogFilter, LoopKindFilter, LoopStatusFilter } from "../../lib/loop-catalog";
import { loopCategories, loopStatuses, matchesLoopFilter } from "../../lib/loop-catalog";
import { loopCatalogFixtures } from "../../mocks/fixtures";

const meta: Meta<typeof LoopCatalog> = {
  title: "systems/loops/components/LoopCatalog",
  component: LoopCatalog,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

const DEFAULT_FILTER: LoopCatalogFilter = { kind: "all", category: null, status: null };

function CatalogHarness({
  initialFilter = DEFAULT_FILTER,
  initialView = "rows" as ListingViewMode,
}: {
  initialFilter?: LoopCatalogFilter;
  initialView?: ListingViewMode;
}) {
  const [filter, setFilter] = useState<LoopCatalogFilter>(initialFilter);
  const [searchQuery, setSearchQuery] = useState("");
  const [view, setView] = useState<ListingViewMode>(initialView);
  const normalizedQuery = searchQuery.trim().toLowerCase();
  const entries = loopCatalogFixtures.filter(
    entry =>
      matchesLoopFilter(entry, filter) &&
      (normalizedQuery === "" ||
        `${entry.name} ${entry.contract.goal}`.toLowerCase().includes(normalizedQuery))
  );

  return (
    <StorySurface className="p-8">
      <div className="mx-auto flex max-w-[1320px] flex-col gap-5">
        <ListingToolbar>
          <ListingToolbar.Leading>
            <ListingToolbar.Search
              aria-label="Search loops"
              onChange={setSearchQuery}
              placeholder="Search loops"
              value={searchQuery}
            />
            <ListingToolbar.Filters>
              <LoopCatalogFilters
                categories={loopCategories(loopCatalogFixtures)}
                categoryFilter={filter.category}
                kindFilter={filter.kind}
                onCategoryFilterChange={(category: string | null) =>
                  setFilter(current => ({ ...current, category }))
                }
                onKindFilterChange={(kind: LoopKindFilter) =>
                  setFilter(current => ({ ...current, kind }))
                }
                onStatusFilterChange={(status: LoopStatusFilter | null) =>
                  setFilter(current => ({ ...current, status }))
                }
                statusFilter={filter.status}
                statuses={loopStatuses(loopCatalogFixtures)}
              />
            </ListingToolbar.Filters>
          </ListingToolbar.Leading>
          <ListingToolbar.Trailing>
            <ListingToolbar.ViewToggle onChange={setView} value={view} />
          </ListingToolbar.Trailing>
        </ListingToolbar>
        <LoopCatalog
          entries={entries}
          hasActiveFilters={
            searchQuery.trim() !== "" ||
            filter.kind !== "all" ||
            filter.category !== null ||
            filter.status !== null
          }
          onClearFilters={() => {
            setFilter(DEFAULT_FILTER);
            setSearchQuery("");
          }}
          onRun={() => {}}
          view={view}
        />
      </div>
    </StorySurface>
  );
}

export const Default: Story = {
  render: () => <CatalogHarness />,
};

export const Cards: Story = {
  render: () => <CatalogHarness initialView="cards" />,
};

export const ReadOnlyOnly: Story = {
  render: () => (
    <CatalogHarness initialFilter={{ kind: "read-only", category: null, status: null }} />
  ),
};
