import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";

export const Route = createFileRoute("/_app/settings/defaults")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Defaults" } },
  }),
  component: createOsRouteSync("settings"),
});
