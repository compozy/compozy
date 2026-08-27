import {
  resolveAppDescriptorForPath as resolveAppForPath,
  SESSION_EMPTY_PATH,
} from "./app-catalog";
import type {
  OsDesktopRuntimeStore,
  OsOpenTarget,
  OsWindowRoute,
  WindowManagerCommandOutcome,
  WindowManagerOpenOutcome,
} from "./os-types";
import { mruWindowInstance } from "./window-instance-lookup";
import { sameOsWindowRoute } from "./window-manager-route";
import { defaultOsWindowRoute } from "./window-manager-view";

const DESKTOP_DEFAULT_VIEW_INTENT_KEY = "_compozy_desktop_default";

function isDesktopDefaultViewIntent(route: OsWindowRoute): boolean {
  const marker = route.search[DESKTOP_DEFAULT_VIEW_INTENT_KEY];
  return route.pathname === "/" && (marker === "1" || marker === 1);
}

function withoutDesktopDefaultViewIntent(route: OsWindowRoute): OsWindowRoute {
  if (!isDesktopDefaultViewIntent(route)) return route;
  const { [DESKTOP_DEFAULT_VIEW_INTENT_KEY]: _intent, ...search } = route.search;
  return { pathname: route.pathname, search };
}

/**
 * The routing coordinator is the ONLY URL↔WM bridge (Safety Invariant 13).
 * Every transition carries an explicit cause:
 *
 * - `route-pop` (browser back/forward, deep link, in-window `<Link>` match):
 *   reconciles URL → store and never writes history.
 * - `user-open` / `user-focus` (dock, palette, rail, window activation): the
 *   store updates first, then exactly one `navigate` (push) reflects the
 *   focused window in the URL.
 * - `hydrate`: the daemon snapshot applies before the initial URL intent; a
 *   cold deep link wins focus without losing the restored desktop.
 * - `workspace-switch`: swap + rehydrate, then one navigate to the target
 *   workspace's focused window route (or `/`).
 */

export interface OsRouterPort {
  /** Pushes one history entry reflecting the location. */
  navigate(route: OsWindowRoute): void;
  /** Truths up hydration or remote authority without manufacturing history. */
  replace(route: OsWindowRoute): void;
}

type RoutingManager = {
  getState(): Pick<OsDesktopRuntimeStore, "client" | "windows" | "focusedId" | "hydration">;
  openOrFocus(target: OsOpenTarget): WindowManagerOpenOutcome;
  closeWindow(windowId: string): Promise<boolean>;
  focusWindow(windowId: string): WindowManagerCommandOutcome;
  restoreWindow(windowId: string): WindowManagerCommandOutcome;
  minimizeWindow(windowId: string): Promise<boolean>;
  zoomWindow(windowId: string): WindowManagerCommandOutcome;
  navigateWindow(
    windowId: string,
    route: OsWindowRoute,
    mode?: "replace" | "push" | "pop"
  ): WindowManagerCommandOutcome;
  retargetWindow(
    windowId: string,
    instanceKey: string,
    route: OsWindowRoute
  ): WindowManagerCommandOutcome;
  popWindowRoute(windowId: string): WindowManagerCommandOutcome;
};

interface RouteReconciliation {
  token: number;
  route: OsWindowRoute;
  desktopDefaultView: boolean;
  inFlight: boolean;
}

export class RoutingCoordinator {
  private readonly manager: RoutingManager;
  private readonly router: OsRouterPort;
  private phase: "hydrating" | "ready" = "hydrating";
  private cycle: "boot" | "workspace-switch" = "boot";
  private initialIntent: OsWindowRoute | null = null;
  private currentRoute: OsWindowRoute | null = null;
  private heldRoute: OsWindowRoute | null = null;
  private desktopDefaultViewIntentPending = false;
  private routeReconciliation: RouteReconciliation | null = null;
  private nextReconciliationToken = 0;
  private pendingNavigateMode: "push" | "pop" | null = null;

  constructor(manager: RoutingManager, router: OsRouterPort) {
    this.manager = manager;
    this.router = router;
  }

  /**
   * Sync-controllers report every matched location here (cause: route-pop).
   * During hydration the location is held as the final focus intent — on a
   * workspace switch this is exactly the cross-workspace deep link that must
   * win over the target workspace's restored focus (US-016.EC-2).
   */
  reportRouteMatch(route: OsWindowRoute): void {
    if (isDesktopDefaultViewIntent(route)) this.desktopDefaultViewIntentPending = true;
    this.currentRoute = route;
    if (this.phase === "hydrating") {
      this.initialIntent = route;
      return;
    }
    this.queueRouteReconciliation(route);
  }

  /**
   * Boot: applies the daemon snapshot's focus truth, then the initial URL as
   * the final focus intent (Routing Model rule 4) — no history write; the URL
   * is trued up by `replace` when a restored focus exists at a neutral URL.
   * Workspace switch: one push to the new workspace's focused route (rule 8) —
   * unless a deep link arrived during the switch, which reconciles instead
   * (its own navigation already wrote the history entry).
   */
  completeHydration(): void {
    if (this.phase !== "hydrating") return;
    this.phase = "ready";
    const intent = this.initialIntent;
    this.initialIntent = null;
    if (this.heldRoute) {
      this.cycle = "boot";
      return;
    }
    if ((intent && isDesktopDefaultViewIntent(intent)) || this.desktopDefaultViewIntentPending) {
      this.cycle = "boot";
      const defaultIntent =
        intent && isDesktopDefaultViewIntent(intent) ? intent : { pathname: "/", search: {} };
      this.replaceRoute(withoutDesktopDefaultViewIntent(defaultIntent));
      this.queueRouteReconciliation(defaultIntent);
      return;
    }
    if (this.cycle === "workspace-switch") {
      this.cycle = "boot";
      if (intent && intent.pathname !== "/") {
        this.queueRouteReconciliation(intent);
        return;
      }
      this.navigateToFocusedOrDesktop();
      return;
    }
    if (intent && intent.pathname !== "/") {
      this.queueRouteReconciliation(intent);
      return;
    }
    const state = this.manager.getState();
    const focused = state.focusedId !== null ? state.windows[state.focusedId] : null;
    if (focused) this.replaceRoute(focused.route);
  }

  /**
   * Daemon/client events report authoritative focus here after their runtime
   * projection is applied. Remote transitions replace the current entry:
   * user actions already own pushes, and repeated snapshot/client frames must
   * not manufacture browser history.
   */
  reportAuthoritativeState(): void {
    if (this.phase !== "ready" || this.heldRoute) return;
    if (this.desktopDefaultViewIntentPending) {
      if (this.routeReconciliation === null) {
        this.queueRouteReconciliation({ pathname: "/", search: {} });
      } else if (!this.routeReconciliation.inFlight) {
        this.reconcilePendingRoute();
      }
      return;
    }
    if (this.routeReconciliation !== null) {
      if (!this.routeReconciliation.inFlight) this.reconcilePendingRoute();
      return;
    }
    const state = this.manager.getState();
    if (state.client === null) return;
    const focused = state.focusedId !== null ? state.windows[state.focusedId] : null;
    if (state.focusedId !== null && !focused) return;
    const route = focused?.route ?? { pathname: "/", search: {} };
    if (sameOsWindowRoute(this.currentRoute, route)) return;
    this.replaceRoute(route);
  }

  /**
   * A routed decision surface owns a URL that deliberately opens no window — today the
   * cross-workspace deep-link confirmation (ADR-004). Hydration and authoritative focus frames
   * would otherwise true the URL up to the focused window and dismiss the decision before the
   * operator answers it, so the hold suspends both until the surface resolves.
   */
  holdRoute(route: OsWindowRoute): void {
    this.invalidateRouteReconciliation();
    this.heldRoute = route;
    this.currentRoute = route;
    this.initialIntent = null;
  }

  releaseRouteHold(): void {
    this.heldRoute = null;
  }

  /**
   * Navigation intent classified where it is known (ADR-011): a drill-in link
   * inside a window body marks `push`, the breadcrumb back marks `pop`. The
   * flag is one-shot — the next route reconciliation consumes it; everything
   * unmarked keeps today's `replace` semantics.
   */
  noteNavigateMode(mode: "push" | "pop"): void {
    this.pendingNavigateMode = mode;
  }

  /**
   * User controls whose destination is safe without a live window-manager
   * client can write URL intent immediately. Route reconciliation keeps that
   * intent pending until the command fence becomes available.
   */
  userNavigate(route: OsWindowRoute): void {
    this.pushRoute(route);
  }

  /** Dock, palette, rail, menubar: open-or-focus then one history entry. */
  async userOpen(target: OsOpenTarget): Promise<string | null> {
    const outcome = this.manager.openOrFocus(target);
    if (!(await outcome.completion)) return null;
    const id = outcome.windowId;
    const route =
      target.route ??
      this.manager.getState().windows[id]?.route ??
      defaultOsWindowRoute(target.app);
    this.pushRoute(route);
    return id;
  }

  /**
   * Window activation (pointerdown, Tab, Enter — Routing Model rule 5). When
   * the activation target is a link, the link's own navigation writes the one
   * history entry and reconciliation follows it (rule 3 coalescing).
   */
  async userFocus(windowId: string, opts: { viaLink?: boolean } = {}): Promise<boolean> {
    const state = this.manager.getState();
    const win = state.windows[windowId];
    const alreadyActive =
      state.focusedId === windowId && !win?.minimized && win?.stackActive === true;
    if (!win || opts.viaLink || alreadyActive) return false;
    const outcome = this.manager.focusWindow(windowId);
    if (!(await outcome.completion)) return false;
    const focused = this.manager.getState().windows[windowId];
    if (focused) this.pushRoute(focused.route);
    return focused !== undefined;
  }

  /**
   * Dock cycling, palette "Go to tab", session rows: land on a specific window
   * in one action (US-017/US-021). Restore carries focus and the desktop
   * switch for the acting client; `window.focus` alone rejects minimized
   * targets, so the two paths are a single decision here.
   */
  async userActivateWindow(windowId: string): Promise<boolean> {
    const state = this.manager.getState();
    const win = state.windows[windowId];
    if (!win) return false;
    const outcome = win.minimized
      ? this.manager.restoreWindow(windowId)
      : this.manager.focusWindow(windowId);
    if (!(await outcome.completion)) return false;
    const focused = this.manager.getState().windows[windowId];
    if (focused) this.pushRoute(focused.route);
    return focused !== undefined;
  }

  /**
   * In-place session/instance switch from inside a window (sidebar rows). The
   * ≤1-window-per-instance invariant wins: when the target instance already
   * owns a live window, that window is activated and reconciled to the target
   * route; otherwise the current window is re-keyed. One history entry follows.
   */
  async userRetarget(
    windowId: string,
    target: { app: OsOpenTarget["app"]; instanceKey: string; route: OsWindowRoute }
  ): Promise<boolean> {
    const state = this.manager.getState();
    const win = state.windows[windowId];
    if (!win) return false;
    const existing = mruWindowInstance(state.windows, state.client?.focusOrder ?? [], {
      app: target.app,
      instanceKey: target.instanceKey,
    });
    if (existing && existing.id !== windowId) {
      if (sameOsWindowRoute(existing.route, target.route)) {
        return this.userActivateWindow(existing.id);
      }
      const outcome = this.manager.openOrFocus(target);
      if (!(await outcome.completion)) return false;
      this.pushRoute(target.route);
      return true;
    }
    if (existing && existing.id === windowId) {
      if (sameOsWindowRoute(win.route, target.route)) return true;
      const navigated = this.manager.navigateWindow(windowId, target.route);
      if (!(await navigated.completion)) return false;
      this.pushRoute(target.route);
      return true;
    }
    const outcome = this.manager.retargetWindow(windowId, target.instanceKey, target.route);
    if (!(await outcome.completion)) return false;
    this.pushRoute(target.route);
    return true;
  }

  /** Drop the session instance and land on the empty route so the frame stays. */
  async userRetireSession(windowId: string): Promise<boolean> {
    const state = this.manager.getState();
    if (!state.windows[windowId]) return false;
    const route = { pathname: SESSION_EMPTY_PATH, search: {} };
    const outcome = this.manager.retargetWindow(windowId, "", route);
    if (!(await outcome.completion)) return false;
    this.pushRoute(route);
    return true;
  }

  /** Close: successor focus follows ADR-002 (next-top window, else desktop). */
  async userClose(windowId: string): Promise<boolean> {
    const state = this.manager.getState();
    if (!state.windows[windowId]) return false;
    const wasFocused = state.focusedId === windowId;
    if (!(await this.manager.closeWindow(windowId))) return false;
    if (wasFocused) this.navigateToFocusedOrDesktop();
    return true;
  }

  /** Minimize: when focus moves to a successor, the URL follows it. */
  async userMinimize(windowId: string): Promise<boolean> {
    const state = this.manager.getState();
    const win = state.windows[windowId];
    if (!win || win.minimized) return false;
    const wasFocused = state.focusedId === windowId;
    if (!(await this.manager.minimizeWindow(windowId))) return false;
    if (wasFocused) this.navigateToFocusedOrDesktop();
    return true;
  }

  /**
   * Zoom already focuses its target in the daemon client projection. For an
   * inactive window, reflect that activation with one history write without
   * preceding the durable command with a competing presentation command.
   */
  async userZoom(windowId: string): Promise<boolean> {
    const state = this.manager.getState();
    const win = state.windows[windowId];
    if (!win || win.minimized) return false;
    const wasFocused = state.focusedId === windowId;
    const outcome = this.manager.zoomWindow(windowId);
    if (!(await outcome.completion)) return false;
    if (!wasFocused) this.pushRoute(win.route);
    return true;
  }

  /**
   * Workspace switch: rehydration restarts; completeHydration navigates once.
   * Route sync can report the current URL before the first workspace bind
   * because layout effects precede the chrome's passive binding effect. Keep
   * that one-shot intent here; completeHydration consumes it, so a later
   * workspace cannot inherit it. Repeated calls during one cycle are no-ops.
   */
  beginWorkspaceSwitch(): void {
    if (this.phase === "hydrating" && this.cycle === "workspace-switch") return;
    this.invalidateRouteReconciliation();
    this.phase = "hydrating";
    this.cycle = "workspace-switch";
  }

  private navigateToFocusedOrDesktop(): void {
    const state = this.manager.getState();
    const focused = state.focusedId !== null ? state.windows[state.focusedId] : null;
    this.pushRoute(focused ? focused.route : { pathname: "/", search: {} });
  }

  private pushRoute(route: OsWindowRoute): void {
    this.desktopDefaultViewIntentPending = false;
    this.currentRoute = route;
    this.router.navigate(route);
  }

  private replaceRoute(route: OsWindowRoute): void {
    this.currentRoute = route;
    this.router.replace(route);
  }

  /**
   * Route-originated reconciliation (rule 2): store updates only, no history.
   * The desktop URL (`/`) opens nothing — it focuses an existing dashboard
   * window or leaves the desktop as-is (first run stays empty, US-001.EC-1).
   */
  private queueRouteReconciliation(route: OsWindowRoute): void {
    const current = this.routeReconciliation;
    const normalizedRoute = withoutDesktopDefaultViewIntent(route);
    const preserveDesktopDefaultView =
      current?.desktopDefaultView === true &&
      sameOsWindowRoute(withoutDesktopDefaultViewIntent(current.route), normalizedRoute);
    const desktopDefaultView =
      isDesktopDefaultViewIntent(route) ||
      (this.desktopDefaultViewIntentPending && normalizedRoute.pathname === "/") ||
      preserveDesktopDefaultView;
    if (current && sameOsWindowRoute(current.route, route)) {
      if (!current.inFlight) this.reconcilePendingRoute();
      return;
    }
    this.nextReconciliationToken += 1;
    this.routeReconciliation = {
      token: this.nextReconciliationToken,
      route,
      desktopDefaultView,
      inFlight: false,
    };
    this.reconcilePendingRoute();
  }

  private reconcilePendingRoute(): void {
    const pending = this.routeReconciliation;
    if (pending === null || pending.inFlight || this.phase !== "ready" || this.heldRoute) return;
    const state = this.manager.getState();
    if (state.client === null || state.hydration !== "live") return;
    const desktopDefaultView = pending.desktopDefaultView;
    const route = withoutDesktopDefaultViewIntent(pending.route);
    const resolved = resolveAppForPath(route.pathname);
    const mode = this.pendingNavigateMode;
    this.pendingNavigateMode = null;
    if (!resolved) {
      this.routeReconciliation = null;
      return;
    }
    const { app, instanceKey } = resolved;
    // Deep links land on the most-recently-used instance of the resolved app
    // (PRD BR-11): with multi-instance apps there is no longer a single window
    // an ID can name.
    const existing = mruWindowInstance(state.windows, state.client?.focusOrder ?? [], {
      app: app.id,
      instanceKey,
    });
    if (route.pathname === "/") {
      if (!existing && !desktopDefaultView) {
        this.routeReconciliation = null;
        return;
      }
      if (
        existing &&
        state.focusedId === existing.id &&
        !existing.minimized &&
        sameOsWindowRoute(existing.route, route)
      ) {
        this.routeReconciliation = null;
        return;
      }
      this.startRouteReconciliation(pending, this.manager.openOrFocus({ app: app.id, route }));
      return;
    }
    if (
      existing &&
      state.focusedId === existing.id &&
      !existing.minimized &&
      sameOsWindowRoute(existing.route, route)
    ) {
      this.routeReconciliation = null;
      return;
    }
    // Breadcrumb back: the durable nav stack owns the destination — the
    // reported route is the app's own rendering of the same pop.
    if (mode === "pop" && existing && !existing.minimized && existing.navStack.length > 0) {
      const outcome = this.manager.popWindowRoute(existing.id);
      this.startRouteReconciliation(pending, { windowId: existing.id, ...outcome });
      return;
    }
    this.startRouteReconciliation(
      pending,
      this.manager.openOrFocus({
        app: app.id,
        instanceKey: instanceKey ?? undefined,
        route,
        ...(mode === "push" ? { navigateMode: "push" as const } : {}),
      })
    );
  }

  private startRouteReconciliation(
    pending: RouteReconciliation,
    outcome: WindowManagerOpenOutcome
  ): void {
    if (!outcome.accepted) {
      if (this.routeReconciliation?.token !== pending.token) return;
      this.routeReconciliation = { ...pending, inFlight: false };
      return;
    }
    this.routeReconciliation = { ...pending, inFlight: true };
    void outcome.completion.then(applied => {
      if (this.routeReconciliation?.token !== pending.token) return;
      if (!applied) {
        this.routeReconciliation = { ...pending, inFlight: false };
        return;
      }
      if (pending.desktopDefaultView) this.desktopDefaultViewIntentPending = false;
      this.routeReconciliation = null;
      this.reportAuthoritativeState();
    });
  }

  private invalidateRouteReconciliation(): void {
    this.nextReconciliationToken += 1;
    this.routeReconciliation = null;
    this.pendingNavigateMode = null;
  }
}
