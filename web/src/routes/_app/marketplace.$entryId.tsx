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
      // The real name arrives through the page's topbar slot; until then show a readable slug.
      crumb: { label: (search.installed_name ?? params.entryId).replaceAll("-", " ") },
    },
  }),
  component: createOsRouteSync("marketplace"),
});
