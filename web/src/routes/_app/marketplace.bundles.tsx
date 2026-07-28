import { createFileRoute } from "@tanstack/react-router";

import { validateMarketplaceKindSearch } from "@/systems/marketplace";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";

const MARKETPLACE_BUNDLES_TOPBAR_CONTEXT: { topbar: TopbarRouteContext } = {
  topbar: { crumb: { label: "Bundles" } },
};

export const Route = createFileRoute("/_app/marketplace/bundles")({
  beforeLoad: (): { topbar: TopbarRouteContext } => MARKETPLACE_BUNDLES_TOPBAR_CONTEXT,
  validateSearch: validateMarketplaceKindSearch,
  component: createOsRouteSync("marketplace"),
});
