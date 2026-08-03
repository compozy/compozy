import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { ListingPage, ListingToolbar, type ListingViewMode } from "@compozy/ui";
import { Repeat2 } from "lucide-react";

import { LoopCatalog } from "../catalog/loop-catalog";
import { LoopCatalogFilters } from "../catalog/loop-catalog-filters";
import { LoopCatalogLede } from "../catalog/loop-catalog-lede";
import type { LoopCatalogFilter, LoopStatusFilter } from "../../lib/loop-catalog";
import { matchesLoopFilter } from "../../lib/loop-catalog";
import { loopCatalogFixtures } from "../../mocks/fixtures";
import { LoopsStoryShell } from "./loops-story-shell";

const meta: Meta<typeof LoopCatalog> = {
  title: "systems/loops/components/LoopCatalog",
  component: LoopCatalog,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

const DEFAULT_FILTER: LoopCatalogFilter = { kind: "all", category: null, status: null };

function CatalogHarness({
  initialStatus = null,
  initialView = "rows" as ListingViewMode,
}: {
  initialStatus?: LoopStatusFilter | null;
  initialView?: ListingViewMode;
}) {
  const [status, setStatus] = useState<LoopStatusFilter | null>(initialStatus);
  const [searchQuery, setSearchQuery] = useState("");
  const [view, setView] = useState<ListingViewMode>(initialView);
  const normalizedQuery = searchQuery.trim().toLowerCase();
  const entries = loopCatalogFixtures.filter(
    entry =>
      matchesLoopFilter(entry, { ...DEFAULT_FILTER, status }) &&
      (normalizedQuery === "" ||
        `${entry.name} ${entry.contract.goal}`.toLowerCase().includes(normalizedQuery))
  );

  const toolbar = (
    <ListingToolbar>
      <ListingToolbar.Leading>
        <ListingToolbar.Search
          aria-label="Search loops"
          onChange={setSearchQuery}
          placeholder="Search loops"
          value={searchQuery}
        />
        <ListingToolbar.Filters>
          <LoopCatalogFilters onStatusFilterChange={setStatus} statusFilter={status} />
        </ListingToolbar.Filters>
      </ListingToolbar.Leading>
      <ListingToolbar.Trailing>
        <ListingToolbar.ViewToggle onChange={setView} value={view} />
      </ListingToolbar.Trailing>
    </ListingToolbar>
  );

  return (
    <LoopsStoryShell crumb="Loops" glyph={<Repeat2 />} toolbar={toolbar}>
      <ListingPage data-testid="loops-catalog">
        <LoopCatalogLede total={entries.length} workspaceLabel="launch-hq" />
        <LoopCatalog
          entries={entries}
          hasActiveFilters={searchQuery.trim() !== "" || status !== null}
          onClearFilters={() => {
            setStatus(null);
            setSearchQuery("");
          }}
          onRun={() => {}}
          view={view}
        />
      </ListingPage>
    </LoopsStoryShell>
  );
}

export const Default: Story = {
  render: () => <CatalogHarness />,
};

export const Cards: Story = {
  render: () => <CatalogHarness initialView="cards" />,
};

/** A status the roster has none of: the truthful empty state plus Clear filters (EC-1). */
export const NoMatchingStatus: Story = {
  render: () => <CatalogHarness initialStatus="canceled" />,
};
