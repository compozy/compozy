import { createFileRoute } from "@tanstack/react-router";

import { validateMarketplaceKindSearch } from "@/systems/marketplace";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";

const MARKETPLACE_MCPS_TOPBAR_CONTEXT: { topbar: TopbarRouteContext } = {
  topbar: { crumb: { label: "MCPs" } },
};

export const Route = createFileRoute("/_app/marketplace/mcps")({
  beforeLoad: (): { topbar: TopbarRouteContext } => MARKETPLACE_MCPS_TOPBAR_CONTEXT,
  validateSearch: validateMarketplaceKindSearch,
  component: createOsRouteSync("marketplace"),
});
