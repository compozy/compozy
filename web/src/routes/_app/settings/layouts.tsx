import { createFileRoute, redirect } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";
import { settingsRouteWithMovedShortcutCommand } from "@/systems/settings";

export interface LayoutsSettingsSearch {
  /** Moved to Shortcuts: a link that still carries it is redirected there. */
  command?: string;
}

export const Route = createFileRoute("/_app/settings/layouts")({
  validateSearch: (search: Record<string, unknown>): LayoutsSettingsSearch => ({
    command:
      typeof search.command === "string" && search.command.trim() !== ""
        ? search.command.trim()
        : undefined,
  }),
  beforeLoad: ({ location, search }): { topbar: TopbarRouteContext } => {
    const moved = settingsRouteWithMovedShortcutCommand({ pathname: location.pathname, search });
    if (moved !== null) {
      throw redirect({ to: moved.pathname, search: { command: search.command }, replace: true });
    }
    return { topbar: { crumb: { label: "Layouts" } } };
  },
  component: createOsRouteSync("settings"),
});
