import { createFileRoute } from "@tanstack/react-router";

import { validateAgentSettingsSearch } from "@/systems/agent/lib/agent-settings-search";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/agents/$name/settings")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Settings" } },
  }),
  validateSearch: validateAgentSettingsSearch,
  loaderDeps: ({ search }) => ({ workspace: search.workspace }),
  loader: async ({ context, params, deps }) =>
    (await import("./-agents-preload")).preloadAgentSettingsRoute(
      context.queryClient,
      params.name,
      deps.workspace
    ),
  component: createOsRouteSync("agents"),
});
