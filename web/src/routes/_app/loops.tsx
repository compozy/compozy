import { createFileRoute } from "@tanstack/react-router";

import { validateLoopsSearch } from "@/systems/os/apps/loops/use-loops-catalog";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadLoopsRoute } from "./-loops-preload";

export const Route = createFileRoute("/_app/loops")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Loops", to: "/loops" } },
  }),
  validateSearch: validateLoopsSearch,
  loaderDeps: ({ search }) => ({
    category: search.category,
    kind:
      search.kind === "read-only"
        ? ("read_only" as const)
        : search.kind === "workspace"
          ? ("workspace" as const)
          : undefined,
    limit: 50,
    q: search.q,
    sort: "name" as const,
    status: search.status,
  }),
  loader: ({ context, deps, location }) =>
    location.pathname.split("/").filter(Boolean).length === 1
      ? preloadLoopsRoute(context.queryClient, deps)
      : Promise.resolve(),
  component: createOsRouteSync("loops"),
});
