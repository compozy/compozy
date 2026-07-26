import { createFileRoute } from "@tanstack/react-router";

import { validateBridgesSearch } from "@/systems/os/apps/bridges/use-bridges-page";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadBridgesRoute } from "./-bridges-preload";

export const Route = createFileRoute("/_app/bridges")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Bridges", to: "/bridges" } },
  }),
  validateSearch: validateBridgesSearch,
  loaderDeps: ({ search }) => ({
    platform: search.platform,
    q: search.q,
    scope: search.scope ?? "all",
    status: search.status,
  }),
  loader: ({ context, deps }) => preloadBridgesRoute(context.queryClient, deps),
  component: createOsRouteSync("bridges"),
});
