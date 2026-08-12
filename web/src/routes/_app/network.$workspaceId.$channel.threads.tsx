import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

interface ThreadsRouteSearch {
  view?: "full";
}

export const Route = createFileRoute("/_app/network/$workspaceId/$channel/threads")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: {
      crumb: {
        label: `#${params.channel} · Threads`,
        params: { channel: params.channel, workspaceId: params.workspaceId },
        to: "/network/$workspaceId/$channel/threads",
      },
    },
  }),
  component: createOsRouteSync("network"),
  loader: async ({ context, params }) =>
    (await import("./-network-preload")).preloadNetworkThreadsRoute(
      context.queryClient,
      params.workspaceId,
      params.channel
    ),
  validateSearch: (search: Record<string, unknown>): ThreadsRouteSearch => ({
    view: search.view === "full" ? "full" : undefined,
  }),
});
