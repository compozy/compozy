import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync, validateMarketplaceDetailSearch } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";

export const Route = createFileRoute("/_app/marketplace/$entryId")({
  validateSearch: validateMarketplaceDetailSearch,
  beforeLoad: ({ params, search }): { topbar: TopbarRouteContext } => ({
    topbar: {
      parentCrumb:
        search.from === "installed"
          ? { label: "Installed", search: { q: search.q }, to: "/marketplace/installed" }
          : { label: "Marketplace", search: { q: search.q }, to: "/marketplace" },
      crumb: { label: params.entryId },
    },
  }),
  component: createOsRouteSync("marketplace"),
});
