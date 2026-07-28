import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadSettingsHooksRoute } from "../-settings-preload";

export const Route = createFileRoute("/_app/settings/hooks")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({ topbar: { crumb: { label: "Hooks" } } }),
  loader: ({ context }) => preloadSettingsHooksRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
