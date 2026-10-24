import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/loops/$name/editor")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Editor" } },
  }),
  loaderDeps: ({ search }) => ({ workspace: search.workspace }),
  loader: async ({ context, deps, params }) =>
    (await import("./-loops-preload")).preloadLoopEditorRoute(
      context.queryClient,
      params.name,
      deps.workspace
    ),
  component: createOsRouteSync("loops"),
});
