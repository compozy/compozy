import type { ComponentType, ReactNode } from "react";

import { WindowScopeContext } from "@/hooks/use-window-scope";

import { useDesktop } from "../hooks/use-desktop";
import type { OsAppDefinition } from "../lib/app-registry";
import type { OsWindowRoute } from "../lib/os-types";

function RoutedFallback({
  Fallback,
  windowId,
}: {
  Fallback: ComponentType<{ route: OsWindowRoute }>;
  windowId: string;
}) {
  const route = useDesktop(state => state.windows[windowId]?.route);
  // Hosted in the window's own scope, the way its surface will be, so a frame that reflects a
  // per-window preference (the session rail's width) reads this window's and not the shell's.
  return route ? (
    <WindowScopeContext value={windowId}>
      <Fallback route={route} />
    </WindowScopeContext>
  ) : null;
}

/**
 * The window body while its app's code loads: the app's own frame at the window's current route,
 * or `generic` for an app that has none. The route is read here, inside the fallback's own
 * subtree, so the window member keeps exactly the subscriptions it has: only a window that is
 * loading pays for this one, and only while it loads.
 */
export function OsWindowLoadingFallback({
  app,
  windowId,
  generic,
}: {
  app: Pick<OsAppDefinition, "LoadingFallback">;
  windowId: string;
  generic: ReactNode;
}) {
  return app.LoadingFallback ? (
    <RoutedFallback Fallback={app.LoadingFallback} windowId={windowId} />
  ) : (
    generic
  );
}
