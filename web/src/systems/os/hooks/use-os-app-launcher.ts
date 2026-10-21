import { settingsSectionPath, type SettingsSectionSlug } from "@/systems/settings";

import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import type { OsWindowRoute } from "../lib/os-types";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

export const USAGE_ROUTE: OsWindowRoute = { pathname: "/usage", search: {} };

export interface OsAppLauncher {
  /** Opens (or focuses) the Usage app. */
  openUsage(): void;
  /** Opens Settings at one section. */
  openSettingsSection(slug: SettingsSectionSlug): void;
}

/**
 * Opens app windows from shell chrome. While window commands are unavailable
 * (degraded shell) the same intent navigates to the app's route instead, so
 * the control still leads somewhere.
 */
export function useOsAppLauncher(): OsAppLauncher {
  const { coordinator } = useOsShell();
  const commandsAvailable = useDesktop(windowManagerCommandsAvailable);
  return {
    openUsage: () => {
      if (commandsAvailable) {
        void coordinator.userOpen({ app: "usage" });
        return;
      }
      coordinator.userNavigate(USAGE_ROUTE);
    },
    openSettingsSection: slug => {
      const route = { pathname: settingsSectionPath(slug), search: {} };
      if (commandsAvailable) {
        void coordinator.userOpen({ app: "settings", route });
        return;
      }
      coordinator.userNavigate(route);
    },
  };
}
