import { useSyncExternalStore } from "react";

import { useAgentCreateHost } from "@/systems/agent";

import { OS_COMPACT_BREAKPOINT, type OsAppId } from "../lib/os-types";
import { useOsShell } from "./use-os-shell";
import { useOsWindowCommands, type OsWindowCommandsModel } from "./use-os-window-commands";

/** Below the compact breakpoint the app menus collapse (US-019.EC-3). */
const COMPACT_QUERY = `(max-width: ${OS_COMPACT_BREAKPOINT - 0.02}px)`;

function subscribeCompact(callback: () => void): () => void {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return () => undefined;
  }
  const mql = window.matchMedia(COMPACT_QUERY);
  if (typeof mql.addEventListener === "function") {
    mql.addEventListener("change", callback);
    return () => mql.removeEventListener("change", callback);
  }
  mql.addListener(callback);
  return () => mql.removeListener(callback);
}

function getCompact(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return false;
  return window.matchMedia(COMPACT_QUERY).matches;
}

export interface MenubarActionsModel {
  /**
   * Whether the app menus render at all. Below the breakpoint they are removed
   * rather than hidden, so the menubar composite never holds an unfocusable
   * item; the palette still carries every action.
   */
  menusVisible: boolean;
  /** The window-manager fence the ⌘K palette uses for the same decisions. */
  canOpenApps: boolean;
  windowCommands: OsWindowCommandsModel;
  openApp(app: OsAppId): void;
  openSettings(): void;
  openAppearance(): void;
  openLayouts(): void;
  openSupport(): void;
  newAgent(): void;
}

/**
 * Menubar view-model: the coordinator-backed navigation actions, the shared
 * window-command model, and the compact-collapse decision. Menu components stay
 * declarative lists and receive every predicate and callback as props.
 */
export function useMenubarActions(): MenubarActionsModel {
  const { coordinator } = useOsShell();
  const agentCreate = useAgentCreateHost();
  const windowCommands = useOsWindowCommands();
  const compact = useSyncExternalStore(subscribeCompact, getCompact, () => false);
  const canOpenApps = windowCommands.commandsAvailable;

  const openApp = (app: OsAppId) => {
    if (canOpenApps) void coordinator.userOpen({ app });
  };
  const openSettingsRoute = (pathname: string) => {
    if (canOpenApps)
      void coordinator.userOpen({ app: "settings", route: { pathname, search: {} } });
  };

  return {
    menusVisible: !compact,
    canOpenApps,
    windowCommands,
    openApp,
    openSettings: () => openApp("settings"),
    openAppearance: () => openSettingsRoute("/settings/appearance"),
    openLayouts: () => openSettingsRoute("/settings/layouts"),
    openSupport: () => openSettingsRoute("/settings/observability"),
    newAgent: () => agentCreate.openDialog(),
  };
}
