import { createFileRoute } from "@tanstack/react-router";

import { validateAgentSettingsSearch } from "@/systems/agent/lib/agent-settings-search";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadAgentSettingsRoute } from "./-agents-preload";

export const Route = createFileRoute("/_app/agents/$name/settings")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Settings" } },
  }),
  validateSearch: validateAgentSettingsSearch,
  loader: ({ context, params }) => preloadAgentSettingsRoute(context.queryClient, params.name),
  component: createOsRouteSync("agents"),
});
