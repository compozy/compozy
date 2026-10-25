import type { ComponentType, ReactNode } from "react";

import { WindowScopeContext } from "@/hooks/use-window-scope";
import { WINDOW_BINDING_SEARCH_KEY, windowDataScope } from "@/systems/workspace";

import { useDesktop } from "../hooks/use-desktop";
import type { OsAppDefinition } from "../lib/app-registry";
import type { OsWindowFallbackProps } from "../lib/os-types";

function RoutedFallback({
  Fallback,
  windowId,
}: {
  Fallback: ComponentType<OsWindowFallbackProps>;
  windowId: string;
}) {
  const route = useDesktop(state => state.windows[windowId]?.route);
  // The window's data scope, the way the loaded window derives it from its desktop and binding.
  const aggregate = useDesktop(
    state =>
      windowDataScope(
        state.snapshot?.workspaceId ?? null,
        state.windows[windowId]?.route.search[WINDOW_BINDING_SEARCH_KEY]
      )?.kind === "all"
  );
  // Hosted in the window's own scope, the way its surface will be, so a frame that reflects a
  // per-window preference (the session view and rail width) reads this window's and not the shell's.
  return route ? (
    <WindowScopeContext value={windowId}>
      <Fallback aggregate={aggregate} route={route} />
    </WindowScopeContext>
  ) : null;
}

/**
 * The window body while its app's code loads: the app's own frame at the window's current route,
 * or `generic` for an app that has none. The route and the window's data scope are read here,
 * inside the fallback's own subtree, so the window member keeps exactly the subscriptions it has
 * and its geometry and normal navigation never depend on them: only a window that is loading pays
 * for these two reads, and only while it loads. The one preference a frame itself may read is its
 * window's own view state (the session view and rail width, from the window scope above), which is what
 * the loaded window mounts from.
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
