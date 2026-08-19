import { createFileRoute } from "@tanstack/react-router";

import { validateLoopRunDiffSearch } from "@/systems/loops";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/loop-runs/$runId/diff")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Compare" } },
  }),

  loaderDeps: ({ search }) => ({
    against_generation: search.against_generation,
    against_run: search.against_run,
    generation: search.generation,
    workspace: search.workspace,
  }),
  loader: async ({ context, deps, params }) =>
    (await import("./-loops-preload")).preloadLoopRunDiffRoute(
      context.queryClient,
      params.runId,
      deps
    ),
  validateSearch: validateLoopRunDiffSearch,
  component: createOsRouteSync("loops"),
});
