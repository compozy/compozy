import { createFileRoute } from "@tanstack/react-router";

import { validateAgentSettingsSearch } from "@/systems/agent/lib/agent-settings-search";
import { createOsRouteSync } from "@/systems/os";

/** Old links only: the agents window maps `section` onto the home, its panels and pages. */
export const Route = createFileRoute("/_app/agents/$name/settings")({
  validateSearch: validateAgentSettingsSearch,
  component: createOsRouteSync("agents"),
});
