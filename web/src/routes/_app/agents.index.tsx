import { createFileRoute } from "@tanstack/react-router";

import { validateAgentsFleetSearch } from "@/systems/agent";
import { createOsRouteSync } from "@/systems/os";

/**
 * The agents fleet is a leaf, not a layout (ADR-008): it alone gives meaning to the fleet search
 * params, so it alone validates them, preloads the team, and reports the fleet window.
 * Matching only `/agents` also removes the pathname inspection the combined route needed to keep
 * descendant navigations from preloading the catalog.
 */
export const Route = createFileRoute("/_app/agents/")({
  validateSearch: validateAgentsFleetSearch,
  loader: async ({ context }) =>
    (await import("./-agents-preload")).preloadAgentsRoute(context.queryClient),
  component: createOsRouteSync("agents"),
});
