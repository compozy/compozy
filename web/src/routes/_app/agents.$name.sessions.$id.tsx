import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import { prefetchAgentSessionRoute } from "./-agent-session-route-loader";

export const Route = createFileRoute("/_app/agents/$name/sessions/$id")({
  beforeLoad: () => ({
    topbar: { crumb: { label: "Session" } },
  }),
  loader: ({ context, params, preload }) =>
    prefetchAgentSessionRoute({
      queryClient: context.queryClient,
      sessionId: params.id,
      preload,
    }),
  component: createOsRouteSync("session"),
});
