import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export interface LayoutsSettingsSearch {
  /** Registry command the shortcut table should land on ("Set shortcut…"). */
  command?: string;
}

export const Route = createFileRoute("/_app/settings/layouts")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Layouts" } },
  }),
  validateSearch: (search: Record<string, unknown>): LayoutsSettingsSearch => ({
    command:
      typeof search.command === "string" && search.command.trim() !== ""
        ? search.command.trim()
        : undefined,
  }),
  component: createOsRouteSync("settings"),
});
