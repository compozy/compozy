import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadHomeRoute } from "./-app-preload";

export const Route = createFileRoute("/_app/")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Home", to: "/" } },
  }),
  loader: ({ context }) => preloadHomeRoute(context.queryClient),
  component: createOsRouteSync("dashboard"),
});
