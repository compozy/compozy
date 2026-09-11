import { useSyncExternalStore } from "react";

import { OS_COMPACT_BREAKPOINT, OS_TOUCH_BREAKPOINT, type OsAppId } from "../lib/os-types";
import { useOsShell } from "./use-os-shell";
import { useOsWindowCommands, type OsWindowCommandsModel } from "./use-os-window-commands";
import { useAgentCreateHost } from "@/systems/agent";
import { settingsSectionPath } from "@/systems/settings";

/** Below the compact breakpoint the app menus collapse (US-019.EC-3). */
const COMPACT_QUERY = `(max-width: ${OS_COMPACT_BREAKPOINT - 0.02}px)`;
/** At or below the touch tier, chrome targets reach the 44px floor (S6/T1). */
const TOUCH_QUERY = `(max-width: ${OS_TOUCH_BREAKPOINT}px)`;

function subscribeQuery(query: string): (callback: () => void) => () => void {
  return callback => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return () => undefined;
    }
    const mql = window.matchMedia(query);
    if (typeof mql.addEventListener === "function") {
      mql.addEventListener("change", callback);
      return () => mql.removeEventListener("change", callback);
    }
    mql.addListener(callback);
    return () => mql.removeListener(callback);
  };
}

function readQuery(query: string): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return false;
  return window.matchMedia(query).matches;
}

const subscribeCompact = subscribeQuery(COMPACT_QUERY);
const getCompact = () => readQuery(COMPACT_QUERY);
const subscribeTouch = subscribeQuery(TOUCH_QUERY);
const getTouch = () => readQuery(TOUCH_QUERY);

export interface MenubarActionsModel {
  /**
   * Whether the app menus render at all. Below the breakpoint they are removed
   * rather than hidden, so the menubar composite never holds an unfocusable
   * item; the palette still carries every action.
   */
  menusVisible: boolean;
  /**
   * Touch tier (S6/T1): at or below `OS_TOUCH_BREAKPOINT` menubar chrome grows
   * to the 44px floor and the identity chip compresses.
   */
  touchViewport: boolean;
  /** The window-manager fence the ⌘K palette uses for the same decisions. */
  canOpenApps: boolean;
  windowCommands: OsWindowCommandsModel;
  openApp(app: OsAppId): void;
  /** Settings → General, where the Updates section lives (ADR-006). */
  openUpdates(): void;
  newAgent(): void;
}

/**
 * Menubar view-model: the compact-collapse decision plus the few affordances
 * that are chrome rather than commands (the update indicator, the agent
 * dialog). Command items project from the registry and dispatch through the
 * seam, so they need nothing from here.
 */
export function useMenubarActions(): MenubarActionsModel {
  const { coordinator } = useOsShell();
  const agentCreate = useAgentCreateHost();
  const windowCommands = useOsWindowCommands();
  const compact = useSyncExternalStore(subscribeCompact, getCompact, () => false);
  const touchViewport = useSyncExternalStore(subscribeTouch, getTouch, () => false);
  const canOpenApps = windowCommands.commandsAvailable;

  const openApp = (app: OsAppId) => {
    if (canOpenApps) void coordinator.userOpen({ app });
  };

  return {
    menusVisible: !compact,
    touchViewport,
    canOpenApps,
    windowCommands,
    openApp,
    openUpdates: () => {
      const route = { pathname: settingsSectionPath("general"), search: {} };
      if (canOpenApps) {
        void coordinator.userOpen({ app: "settings", route });
        return;
      }
      coordinator.userNavigate(route);
    },
    newAgent: () => agentCreate.openDialog(),
  };
}
