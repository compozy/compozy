import { createFileRoute } from "@tanstack/react-router";

import { validateMarketplaceKindSearch } from "@/systems/marketplace";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";

const MARKETPLACE_EXTENSIONS_TOPBAR_CONTEXT: { topbar: TopbarRouteContext } = {
  topbar: { crumb: { label: "Extensions" } },
};

export const Route = createFileRoute("/_app/marketplace/extensions")({
  beforeLoad: (): { topbar: TopbarRouteContext } => MARKETPLACE_EXTENSIONS_TOPBAR_CONTEXT,
  validateSearch: validateMarketplaceKindSearch,
  component: createOsRouteSync("marketplace"),
});
