import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadNetworkThreadDetailRoute } from "./-network-preload";

interface ThreadDetailSearch {
  view?: "full";
}

export const Route = createFileRoute("/_app/network/$workspaceId/$channel/threads/$threadId")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: `#${params.channel} · Thread` } },
  }),
  component: createOsRouteSync("network"),
  loader: ({ context, params }) =>
    preloadNetworkThreadDetailRoute(
      context.queryClient,
      params.workspaceId,
      params.channel,
      params.threadId
    ),
  validateSearch: (search: Record<string, unknown>): ThreadDetailSearch => ({
    view: search.view === "full" ? "full" : undefined,
  }),
});
