import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/appearance")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Appearance" } },
  }),
  component: createOsRouteSync("settings"),
});
