import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/loops/$name/editor")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Editor" } },
  }),
  loader: async ({ context, params }) =>
    (await import("./-loops-preload")).preloadLoopEditorRoute(context.queryClient, params.name),
  component: createOsRouteSync("loops"),
});
