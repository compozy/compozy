import { createFileRoute, redirect } from "@tanstack/react-router";
import { toast } from "sonner";

import { createOsRouteSync } from "@/systems/os";
import { prefetchAgentSessionRoute } from "./-agent-session-route-loader";

export const Route = createFileRoute("/_app/agents/$name/sessions/$id")({
  beforeLoad: () => ({
    topbar: { crumb: { label: "Session" } },
  }),
  loader: async ({ context, params, preload }) => {
    const data = await prefetchAgentSessionRoute({
      queryClient: context.queryClient,
      sessionId: params.id,
    });
    if (!preload && data.workspaceId === null) {
      toast.error("Session not found");
      throw redirect({
        to: "/agents/$name",
        params: { name: params.name },
        replace: true,
      });
    }
    return data;
  },
  component: createOsRouteSync("session"),
});
