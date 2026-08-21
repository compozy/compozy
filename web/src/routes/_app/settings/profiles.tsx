import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";
import { validateProfilesSettingsSearch } from "@/systems/profiles";

export const Route = createFileRoute("/_app/settings/profiles")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Profiles" } },
  }),
  validateSearch: validateProfilesSettingsSearch,
  component: createOsRouteSync("settings"),
});
