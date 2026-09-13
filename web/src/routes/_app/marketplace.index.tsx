import { createFileRoute, redirect } from "@tanstack/react-router";

import { validateMarketplaceSearch } from "@/systems/marketplace";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/marketplace/")({
  validateSearch: validateMarketplaceSearch,
  beforeLoad: ({ location, search }) => {
    if (Object.hasOwn(location.search, "tab")) {
      throw redirect({ to: "/marketplace", search: { q: search.q }, replace: true });
    }
  },
  component: createOsRouteSync("marketplace"),
});
