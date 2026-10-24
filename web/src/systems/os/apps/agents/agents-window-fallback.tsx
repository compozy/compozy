import { cn, LaneTabs, ListingPage, SkeletonRegion, type ListingViewMode } from "@compozy/ui";

import {
  ToolbarSearchSkeleton,
  ToolbarSlotSkeleton,
  ToolbarViewToggleSkeleton,
} from "../../components/os-window-toolbar-skeleton";
import type { OsWindowRoute } from "../../lib/os-types";
import {
  AGENT_DETAIL_BODY_CLASS,
  AGENT_DETAIL_PAGE_CLASS,
  AGENT_DETAIL_ROOT_CLASS,
  AGENT_DETAIL_TABS_CLASS,
  AGENT_TAB_PANEL_CLASS,
} from "./agent-detail-frame-classes";
import { agentDetailTabs } from "./agent-detail-view";
import { parseAgentWindowLocation } from "./agent-window-location";
import {
  AgentConfigurationSkeleton,
  AgentDetailHeaderSkeleton,
  AgentFleetList,
  AgentInstructionsPlaceholder,
  AgentOverviewSkeleton,
  AgentSessionsTabSkeleton,
  resolveAgentDetailSearch,
  type AgentDetailTab,
  type AgentDetailSearch,
} from "@/systems/agent";

const noop = () => undefined;

/**
 * The listing before its first read: the strip without Filter (which the fleet only offers once
 * there is something to filter), over the fleet's loading rows or cards.
 */
function AgentsCatalogFallback({ view }: { view: ListingViewMode }) {
  return (
    <>
      <ToolbarSlotSkeleton
        leading={<ToolbarSearchSkeleton />}
        trailing={<ToolbarViewToggleSkeleton />}
      />
      <ListingPage data-testid="agent-fleet-page">
        <AgentFleetList
          onClearFilters={noop}
          onCreateAgent={noop}
          onLoadMore={noop}
          onNewSession={noop}
          rows={[]}
          sessionDataStatus="available"
          status="loading"
          view={view}
        />
      </ListingPage>
    </>
  );
}

function AgentTabSkeleton({ search }: { search: ReturnType<typeof resolveAgentDetailSearch> }) {
  switch (search.tab) {
    case "overview":
      return (
        <SkeletonRegion label="Loading agent">
          <AgentOverviewSkeleton />
        </SkeletonRegion>
      );
    case "configuration":
      return (
        <SkeletonRegion label="Loading configuration">
          <AgentConfigurationSkeleton />
        </SkeletonRegion>
      );
    case "instructions":
      return (
        <SkeletonRegion label="Loading instructions">
          <AgentInstructionsPlaceholder file={search.file} />
        </SkeletonRegion>
      );
    case "sessions":
      return <AgentSessionsTabSkeleton filter={search.filter} inert onFilterChange={noop} />;
  }
}

/**
 * The agent page while its record loads, the way the location shows it, with the tab list out of
 * reach: the page frame, the header's meta row, the tabs and the open tab's loading shape.
 */
function AgentDetailFallback({ search }: { search: AgentDetailSearch }) {
  const resolved = resolveAgentDetailSearch(search);
  return (
    <div className={AGENT_DETAIL_ROOT_CLASS} data-testid="agent-detail-loading">
      <div className={AGENT_DETAIL_PAGE_CLASS}>
        <AgentDetailHeaderSkeleton />
        <div className={cn("flex flex-col", AGENT_DETAIL_TABS_CLASS)}>
          <div className="contents" inert>
            <LaneTabs<AgentDetailTab>
              ariaLabel="Agent detail panels"
              className="gap-0"
              items={agentDetailTabs("pending")}
              listClassName="w-full"
              onChange={noop}
              value={resolved.tab}
            />
          </div>
          <div className={AGENT_DETAIL_BODY_CLASS} data-testid="agent-detail-body">
            <div className={cn("flex-1 text-small-body", AGENT_TAB_PANEL_CLASS)}>
              <AgentTabSkeleton search={resolved} />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/**
 * The agents window's body while its code loads: the surface the route is about to show, in the
 * loading state its location shows while that surface's read is out. The settings route is a
 * dialog over the agent page, so its body is the agent page.
 */
export function AgentsWindowFallback({ route }: { route: OsWindowRoute }) {
  const location = parseAgentWindowLocation(route);
  if (location.kind === "settings") return <AgentDetailFallback search={{}} />;
  if (location.kind === "detail") return <AgentDetailFallback search={location.search} />;
  return <AgentsCatalogFallback view={location.search.view ?? "rows"} />;
}
