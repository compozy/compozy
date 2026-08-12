import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

interface ThreadDetailSearch {
  view?: "full";
}

export const Route = createFileRoute("/_app/network/$workspaceId/$channel/threads/$threadId")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: `#${params.channel} · Thread` } },
  }),
  component: createOsRouteSync("network"),
  loader: async ({ context, params }) =>
    (await import("./-network-preload")).preloadNetworkThreadDetailRoute(
      context.queryClient,
      params.workspaceId,
      params.channel,
      params.threadId
    ),
  validateSearch: (search: Record<string, unknown>): ThreadDetailSearch => ({
    view: search.view === "full" ? "full" : undefined,
  }),
});
