import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Home", to: "/" } },
  }),
  loader: async ({ context }) =>
    (await import("./-home-preload")).preloadHomeRoute(context.queryClient),
  component: createOsRouteSync("dashboard"),
});
