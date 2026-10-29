import { createFileRoute } from "@tanstack/react-router";

import { validateAgentDetailSearch } from "@/systems/agent";
import { createOsRouteSync } from "@/systems/os";

/**
 * Agent detail is a leaf, not a layout (ADR-008): it is the only route that gives meaning to the
 * agent-detail search params, so it is the only route that validates them, preloads for them, and
 * reports the agent window to the OS coordinator.
 */
export const Route = createFileRoute("/_app/agents/$name/")({
  validateSearch: validateAgentDetailSearch,
  loaderDeps: ({ search }) => ({ workspace: search.workspace }),
  loader: async ({ context, params, deps }) =>
    (await import("./-agents-preload")).preloadAgentDetailRoute(
      context.queryClient,
      params.name,
      deps.workspace
    ),
  component: createOsRouteSync("agents"),
});
