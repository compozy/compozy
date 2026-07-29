import { createFileRoute } from "@tanstack/react-router";

import { validateAgentDetailSearch } from "@/systems/agent";
import { createOsRouteSync } from "@/systems/os";
import { preloadAgentDetailRoute } from "./-app-preload";

/**
 * Agent detail is a leaf, not a layout (ADR-008): it is the only route that gives meaning to the
 * agent-detail search params, so it is the only route that validates them, preloads for them, and
 * reports the agent window to the OS coordinator.
 */
export const Route = createFileRoute("/_app/agents/$name/")({
  validateSearch: validateAgentDetailSearch,
  loader: ({ context, params }) => preloadAgentDetailRoute(context.queryClient, params.name),
  component: createOsRouteSync("agents"),
});
