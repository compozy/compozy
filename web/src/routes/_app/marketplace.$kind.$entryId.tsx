import { createFileRoute } from "@tanstack/react-router";

import {
  MARKETPLACE_KIND_LABEL,
  isMarketplaceKind,
  marketplaceRouteKindFor,
} from "@/systems/marketplace";
import { createOsRouteSync } from "@/systems/os";
import { validateMarketplaceDetailSearch } from "@/systems/os/apps/marketplace/marketplace-detail-search";
import type { TopbarRouteContext } from "@/types/topbar";

export const Route = createFileRoute("/_app/marketplace/$kind/$entryId")({
  validateSearch: validateMarketplaceDetailSearch,
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: {
      parentCrumb: isMarketplaceKind(params.kind)
        ? {
            label: MARKETPLACE_KIND_LABEL[params.kind],
            to: `/marketplace/${marketplaceRouteKindFor(params.kind)}`,
          }
        : undefined,
      crumb: { label: params.entryId },
    },
  }),
  component: createOsRouteSync("marketplace"),
});
