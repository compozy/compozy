import { createFileRoute } from "@tanstack/react-router";

import { validateMarketplaceSearch } from "@/systems/marketplace";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/marketplace/")({
  validateSearch: validateMarketplaceSearch,
  component: createOsRouteSync("marketplace"),
});
