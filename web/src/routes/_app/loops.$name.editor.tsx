import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadLoopEditorRoute } from "./-loops-preload";

export const Route = createFileRoute("/_app/loops/$name/editor")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Editor" } },
  }),
  loader: ({ context, params }) => preloadLoopEditorRoute(context.queryClient, params.name),
  component: createOsRouteSync("loops"),
});
