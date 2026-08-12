import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/skills")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Skills" } },
  }),
  loader: async ({ context }) =>
    (await import("../-settings-preload")).preloadSettingsSkillsRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
