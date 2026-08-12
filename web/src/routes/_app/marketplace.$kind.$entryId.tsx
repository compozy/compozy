import { createFileRoute } from "@tanstack/react-router";

import { validateMarketplaceDetailSearch } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import {
  isMarketplaceKind,
  MARKETPLACE_KIND_LABEL,
  marketplaceRouteKindFor,
} from "@/systems/marketplace";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/marketplace/$kind/$entryId")({
  validateSearch: validateMarketplaceDetailSearch,
  beforeLoad: ({ params, search }): { topbar: TopbarRouteContext } => ({
    topbar: {
      parentCrumb: isMarketplaceKind(params.kind)
        ? {
            label: MARKETPLACE_KIND_LABEL[params.kind],
            search: { scope: search.scope, tab: search.tab, workspace_id: search.workspace_id },
            to: `/marketplace/${marketplaceRouteKindFor(params.kind)}`,
          }
        : undefined,
      crumb: { label: params.entryId },
    },
  }),
  component: createOsRouteSync("marketplace"),
});
