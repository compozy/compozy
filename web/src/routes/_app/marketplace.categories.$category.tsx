import { createFileRoute } from "@tanstack/react-router";

import { validateMarketplaceSearch } from "@/systems/marketplace";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";

export const Route = createFileRoute("/_app/marketplace/categories/$category")({
  validateSearch: validateMarketplaceSearch,
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: {
      parentCrumb: { label: "Marketplace", to: "/marketplace" },
      // The daemon label arrives through the page's topbar slot; until then show the id.
      crumb: { label: params.category },
    },
  }),
  component: createOsRouteSync("marketplace"),
});
