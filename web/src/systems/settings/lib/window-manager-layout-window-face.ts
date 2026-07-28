import { AppWindow } from "lucide-react";

import { OS_APPS, getOsApp, type OsAppId } from "@/systems/os";

import type { WindowManagerLayoutWindow } from "./window-manager-layout-types";

export interface LayoutWindowFace {
  title: string;
  detail: string;
  icon: typeof AppWindow;
}

function osAppId(value: string): OsAppId | null {
  return Object.hasOwn(OS_APPS, value) ? (value as OsAppId) : null;
}

/**
 * How a window reads on the canvas. Titles come from the app registry — the same
 * source the shell's window frames use — so a tile is labelled with the words the
 * person already sees on the real window, not with its id.
 */
export function layoutWindowFace(
  window: WindowManagerLayoutWindow | undefined,
  windowId: string
): LayoutWindowFace {
  if (!window) {
    return { title: windowId, detail: "Not in this workspace", icon: AppWindow };
  }
  const appId = osAppId(window.app);
  const app = appId === null ? null : getOsApp(appId);
  return {
    title: app?.title ?? window.app,
    detail: window.minimized ? "Minimized" : window.route.pathname,
    icon: app?.icon ?? AppWindow,
  };
}
