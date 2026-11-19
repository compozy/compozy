import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export interface ShortcutsSettingsSearch {
  /** Registry command the shortcut table should land on ("Set shortcut…"). */
  command?: string;
}

export const Route = createFileRoute("/_app/settings/shortcuts")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Shortcuts" } },
  }),
  validateSearch: (search: Record<string, unknown>): ShortcutsSettingsSearch => ({
    command:
      typeof search.command === "string" && search.command.trim() !== ""
        ? search.command.trim()
        : undefined,
  }),
  component: createOsRouteSync("settings"),
});
