import { createFileRoute } from "@tanstack/react-router";

import { validateAgentDetailSearch } from "@/systems/agent/lib/agent-detail-search";
import { createOsRouteSync } from "@/systems/os";

/** A focused page of the agent: What it does, Check-ins, Memory, or Where it works. */
export const Route = createFileRoute("/_app/agents/$name/$page")({
  validateSearch: validateAgentDetailSearch,
  loaderDeps: ({ search }) => ({ workspace: search.workspace }),
  loader: async ({ context, params, deps }) =>
    (await import("./-agents-preload")).preloadAgentSettingsRoute(
      context.queryClient,
      params.name,
      deps.workspace
    ),
  component: createOsRouteSync("agents"),
});
