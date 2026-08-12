import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

const MARKETPLACE_TOPBAR_CONTEXT: { topbar: TopbarRouteContext } = {
  topbar: { crumb: { label: "Marketplace", to: "/marketplace" } },
};

export const Route = createFileRoute("/_app/marketplace")({
  beforeLoad: (): { topbar: TopbarRouteContext } => MARKETPLACE_TOPBAR_CONTEXT,
  component: createOsRouteSync("marketplace"),
});
