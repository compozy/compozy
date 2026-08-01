import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";

export const Route = createFileRoute("/_app/new-tab")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "New tab", to: "/new-tab" } },
  }),
  component: createOsRouteSync("new-tab"),
});
