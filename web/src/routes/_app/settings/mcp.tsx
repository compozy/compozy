import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/mcp")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "MCP servers" } },
  }),
  loader: async ({ context }) =>
    (await import("../-settings-preload")).preloadSettingsMCPRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
