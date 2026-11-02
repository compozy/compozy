import { ListingPage, type ListingViewMode } from "@compozy/ui";

import {
  ToolbarSearchSkeleton,
  ToolbarSlotSkeleton,
  ToolbarViewToggleSkeleton,
} from "../../components/os-window-toolbar-skeleton";
import type { OsWindowRoute } from "../../lib/os-types";
import { AgentDetailLoading } from "./agent-detail-location";
import { parseAgentWindowLocation } from "./agent-window-location";
import { AgentFleetSkeleton } from "@/systems/agent";

/**
 * The listing before its first read: the strip without Filter (which the fleet only offers once
 * there is something to filter), over the team's loading rows or cards.
 */
function AgentsCatalogFallback({ view }: { view: ListingViewMode }) {
  return (
    <>
      <ToolbarSlotSkeleton
        leading={<ToolbarSearchSkeleton />}
        trailing={<ToolbarViewToggleSkeleton />}
      />
      <ListingPage data-testid="agent-fleet-page">
        <AgentFleetSkeleton view={view} />
      </ListingPage>
    </>
  );
}

/**
 * The agents window's body while its code loads: the surface the route is about to show, in the
 * loading state its location shows while that surface's read is out.
 */
export function AgentsWindowFallback({ route }: { route: OsWindowRoute }) {
  const location = parseAgentWindowLocation(route);
  if (location.kind !== "catalog") return <AgentDetailLoading />;
  return <AgentsCatalogFallback view={location.search.view ?? "cards"} />;
}
