import type { resolveAppDescriptorForPath } from "./app-catalog";
import { createdSessionHostWindow, landCreatedSession } from "./created-session-window";
import type { OsWindowRoute, WindowManagerOpenOutcome } from "./os-types";
import type { RoutingManager } from "./routing-coordinator-contract";
import { mruWindowInstance } from "./window-instance-lookup";
import { sameOsWindowRoute } from "./window-manager-route";

export interface RouteDecisionInput {
  resolved: NonNullable<ReturnType<typeof resolveAppDescriptorForPath>>;
  route: OsWindowRoute;
  /** The explicit desktop default-view intent may reach a dashboard on another desktop. */
  desktopDefaultView: boolean;
  /** One-shot navigation intent classified where it was known (ADR-011). */
  mode: "push" | "pop" | null;
  /** The route names a session the operator just created (#825). */
  createdSession: boolean;
}

/**
 * What a route-originated reconciliation (Routing Model rule 2) asks of the window manager:
 * the command to wait on, or `null` when the store already shows the route — or, for the
 * desktop URL, must not change. Never writes history.
 */
export function decideRouteReconciliation(
  manager: RoutingManager,
  { resolved, route, desktopDefaultView, mode, createdSession }: RouteDecisionInput
): WindowManagerOpenOutcome | null {
  const state = manager.getState();
  const { app, instanceKey } = resolved;
  // Deep links land on the most-recently-used instance of the resolved app
  // (PRD BR-11): with multi-instance apps there is no longer a single window
  // an ID can name.
  const existing = mruWindowInstance(state.windows, state.client?.focusOrder ?? [], {
    app: app.id,
    instanceKey,
  });
  const alreadyShown =
    existing !== null &&
    state.focusedId === existing.id &&
    !existing.minimized &&
    sameOsWindowRoute(existing.route, route);
  if (route.pathname === "/") {
    // A bare `/` is where an empty desktop lands. Reusing a dashboard on
    // another desktop would focus it and switch the client straight back, so
    // only the explicit default-view intent may reach across desktops.
    const onActiveDesktop = existing?.desktopId === (state.client?.activeDesktopId ?? null);
    if ((!desktopDefaultView && !onActiveDesktop) || alreadyShown) return null;
    return manager.openOrFocus({ app: app.id, route });
  }
  if (alreadyShown) return null;
  // A deep link can leave a different parent in history. Consume the stack
  // only when it matches the breadcrumb's selected destination.
  const canPop = sameOsWindowRoute(existing?.navStack.at(-1) ?? null, route);
  if (mode === "pop" && existing && !existing.minimized && canPop) {
    return { windowId: existing.id, ...manager.popWindowRoute(existing.id, route) };
  }
  if (createdSession && app.id === "session" && instanceKey) {
    const host = createdSessionHostWindow(state, { instanceKey, route });
    if (host) return landCreatedSession(manager, host, { instanceKey, route });
  }
  return manager.openOrFocus({
    app: app.id,
    instanceKey: instanceKey ?? undefined,
    route,
    ...(mode === "push" ? { navigateMode: "push" as const } : {}),
  });
}
