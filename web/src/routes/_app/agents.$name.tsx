import { createFileRoute } from "@tanstack/react-router";

import { validateAgentDetailSearch } from "@/systems/agent";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadAgentDetailRoute } from "./-app-preload";

export const Route = createFileRoute("/_app/agents/$name")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: params.name, params: { name: params.name }, to: "/agents/$name" } },
  }),
  validateSearch: validateAgentDetailSearch,
  loader: ({ context, params }) => preloadAgentDetailRoute(context.queryClient, params.name),
  component: createOsRouteSync("agents"),
});
