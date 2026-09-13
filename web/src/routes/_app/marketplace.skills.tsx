import { createFileRoute, redirect } from "@tanstack/react-router";

import { validateMarketplaceSearch } from "@/systems/marketplace";

/** Retired kind path: redirects to the one catalog for one release (`?tab=` is dropped). */
export const Route = createFileRoute("/_app/marketplace/skills")({
  validateSearch: validateMarketplaceSearch,
  beforeLoad: ({ search }) => {
    throw redirect({ search: { q: search.q }, to: "/marketplace" });
  },
  component: RetiredMarketplaceKindRedirect,
});

function RetiredMarketplaceKindRedirect() {
  return null;
}
