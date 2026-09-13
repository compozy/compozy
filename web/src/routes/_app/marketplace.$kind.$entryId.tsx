import { createFileRoute, redirect } from "@tanstack/react-router";

import { validateMarketplaceDetailSearch } from "@/systems/os";

/**
 * Retired kind detail path: redirects to the one-catalog entry for one release, keeping the
 * installed identity and scope of the row that linked here.
 */
export const Route = createFileRoute("/_app/marketplace/$kind/$entryId")({
  validateSearch: validateMarketplaceDetailSearch,
  beforeLoad: ({ params, search }) => {
    throw redirect({
      params: { entryId: params.entryId },
      search: {
        installed_name: search.installed_name,
        profile: search.profile,
        scope: search.scope,
        workspace_id: search.workspace_id,
      },
      to: "/marketplace/$entryId",
    });
  },
  component: RetiredMarketplaceDetailRedirect,
});

function RetiredMarketplaceDetailRedirect() {
  return null;
}
