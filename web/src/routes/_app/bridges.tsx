import { createFileRoute } from "@tanstack/react-router";

import { validateBridgesSearch } from "@/systems/bridges";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/bridges")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Connections", to: "/bridges" } },
  }),
  validateSearch: validateBridgesSearch,
  loaderDeps: ({ search }) => ({
    platform: search.platform,
    q: search.q,
    scope: search.scope ?? "all",
    status: search.status,
  }),
  loader: async ({ context, deps }) =>
    (await import("./-bridges-preload")).preloadBridgesRoute(context.queryClient, deps),
  component: createOsRouteSync("bridges"),
});
