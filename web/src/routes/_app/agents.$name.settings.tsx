import { createFileRoute } from "@tanstack/react-router";

import { validateAgentSettingsSearch } from "@/systems/agent/lib/agent-settings-search";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/agents/$name/settings")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Settings" } },
  }),
  validateSearch: validateAgentSettingsSearch,
  loader: async ({ context, params }) =>
    (await import("./-agents-preload")).preloadAgentSettingsRoute(context.queryClient, params.name),
  component: createOsRouteSync("agents"),
});
