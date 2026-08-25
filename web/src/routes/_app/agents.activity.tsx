import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";

/**
 * The Activity location of the Agents app.
 *
 * A static sibling of `/agents/$name`, and the router prefers it over the
 * dynamic segment — which is what stops "activity" from being read as the name
 * of an agent. The window controller applies the same ordering when it parses
 * the WM location.
 *
 * No search params: the tree is scoped by the shell's own workspace and profile
 * lenses, and adding a second way to scope it would be a second source of truth.
 */
export const Route = createFileRoute("/_app/agents/activity")({
  component: createOsRouteSync("agents"),
});
