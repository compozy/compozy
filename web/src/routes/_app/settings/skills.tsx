import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadSettingsSkillsRoute } from "../-settings-preload";

export const Route = createFileRoute("/_app/settings/skills")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Skills" } },
  }),
  loader: ({ context }) => preloadSettingsSkillsRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
