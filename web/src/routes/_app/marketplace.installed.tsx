import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { validateMarketplaceSearch } from "@/systems/marketplace";
import { createOsRouteSync } from "@/systems/os";

const MARKETPLACE_INSTALLED_TOPBAR_CONTEXT: { topbar: TopbarRouteContext } = {
  topbar: { crumb: { label: "Installed" } },
};

export const Route = createFileRoute("/_app/marketplace/installed")({
  beforeLoad: (): { topbar: TopbarRouteContext } => MARKETPLACE_INSTALLED_TOPBAR_CONTEXT,
  validateSearch: validateMarketplaceSearch,
  component: createOsRouteSync("marketplace"),
});
