import {
  resolveAppDescriptorForPath as resolveAppForPath,
  SESSION_EMPTY_PATH,
} from "./app-catalog";
import { createdSessionHostWindow, landCreatedSession } from "./created-session-window";
import type { OsOpenTarget, OsWindowRoute, WindowManagerOpenOutcome } from "./os-types";
import { mruWindowInstance } from "./window-instance-lookup";
import type {
  OsRouterPort,
  RouteReconciliation,
  RoutingManager,
} from "./routing-coordinator-contract";
import {
  carriesDesktopDefaultViewIntent,
  isDesktopDefaultViewIntent,
  withoutDesktopDefaultViewIntent,
} from "./routing-default-view-intent";
import { decideRouteReconciliation } from "./routing-route-decision";
import { sameOsWindowRoute } from "./window-manager-route";
import { defaultOsWindowRoute } from "./window-manager-view";

export type { OsRouterPort } from "./routing-coordinator-contract";

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
  private nextUserRetargetToken = 0;
  /**
   * Bumped by every navigation the coordinator did not write itself and by every explicit
   * user navigation or focus choice. A user command still waiting on the daemon compares its
   * token before writing history or taking focus: a newer intent already owns the URL, and a
   * late push would drag the operator back to a destination they left (rule 3 coalescing).
   */
  private nextUserIntentToken = 0;
  private pendingNavigateMode: "push" | "pop" | null = null;
  private createdSessionIntent: { instanceKey: string; afterToken: number } | null = null;

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
    // The coordinator sets `currentRoute` before its own pushes and replaces, so
    // a report that differs is external intent: a link, back/forward, a deep link.
    if (!sameOsWindowRoute(route, this.currentRoute)) this.nextUserIntentToken += 1;
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
   * The New session dialog names its session before navigating, so the reconciliation that
   * lands it prefers an existing Session window (#825). Only that route or a newer
   * reconciliation consumes the hint; it survives a workspace switch (held deep link).
   */
  noteCreatedSession(sessionId: string): void {
    const instanceKey = sessionId.trim();
    const afterToken = this.nextReconciliationToken;
    this.createdSessionIntent = instanceKey ? { instanceKey, afterToken } : null;
  }

  /**
   * User controls whose destination is safe without a live window-manager
   * client can write URL intent immediately. Route reconciliation keeps that
   * intent pending until the command fence becomes available.
   *
   * The reconciliation is queued here as well as by the sync-controller: the
   * router treats a navigation to the current location as a no-op, so a window
   * whose projection has drifted from the URL (an optimistic navigate still
   * waiting on the daemon) would otherwise never hear about this gesture.
   */
  userNavigate(route: OsWindowRoute): void {
    this.nextUserIntentToken += 1;
    this.pushRoute(route);
    if (this.phase === "ready") this.queueRouteReconciliation(route);
  }

  /** Dock, palette, rail, menubar: open-or-focus then one history entry. */
  async userOpen(target: OsOpenTarget): Promise<string | null> {
    const intentToken = ++this.nextUserIntentToken;
    const outcome = this.manager.openOrFocus(target);
    if (!(await outcome.completion)) return null;
    const id = outcome.windowId;
    // A newer navigation arrived while the daemon held this open; it owns the
    // URL now and the window already follows it.
    if (intentToken !== this.nextUserIntentToken) return id;
    // A window can consume its route intent before the opening command completes.
    const route =
      this.manager.getState().windows[id]?.route ??
      target.route ??
      defaultOsWindowRoute(target.app);
    this.pushRoute(route);
    return id;
  }

  /** Launchers that open a just-created session directly: prefer an existing Session window. */
  async userOpenCreatedSession(
    target: OsOpenTarget & { instanceKey: string; route: OsWindowRoute }
  ): Promise<string | null> {
    const host = createdSessionHostWindow(this.manager.getState(), target);
    if (host === null) return this.userOpen(target);
    const intentToken = ++this.nextUserIntentToken;
    const isCurrent = () => intentToken === this.nextUserIntentToken;
    const outcome = landCreatedSession(this.manager, host, target, isCurrent);
    if (await outcome.completion) {
      if (isCurrent()) this.pushRoute(target.route);
      return host.id;
    }
    // Unless a newer gesture owns focus: a host another client closed means choose again (a new
    // window when none is left); a surviving host that refused the landing opens it the usual way.
    if (!isCurrent()) return null;
    if (!this.manager.getState().windows[host.id]) return this.userOpenCreatedSession(target);
    return this.userOpen(target);
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
    if (!win || opts.viaLink) return false;
    this.nextUserIntentToken += 1;
    if (alreadyActive) return false;
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
    this.nextUserIntentToken += 1;
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
    const retargetToken = ++this.nextUserRetargetToken;
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
      if (retargetToken === this.nextUserRetargetToken) this.pushRoute(target.route);
      return true;
    }
    if (existing && existing.id === windowId) {
      if (sameOsWindowRoute(win.route, target.route)) return true;
      const navigated = this.manager.navigateWindow(windowId, target.route);
      if (!(await navigated.completion)) return false;
      if (retargetToken === this.nextUserRetargetToken) this.pushRoute(target.route);
      return true;
    }
    const outcome = this.manager.retargetWindow(windowId, target.instanceKey, target.route);
    if (!(await outcome.completion)) return false;
    if (retargetToken === this.nextUserRetargetToken) this.pushRoute(target.route);
    return true;
  }

  /** Drop `sessionId`'s instance, unless the window moved on; the frame stays on the empty route. */
  async userRetireSession(windowId: string, sessionId?: string): Promise<boolean> {
    const win = this.manager.getState().windows[windowId];
    if (!win || (sessionId !== undefined && win.instanceKey !== sessionId)) return false;
    const route = { pathname: SESSION_EMPTY_PATH, search: {} };
    const outcome = this.manager.retargetWindow(windowId, "", route);
    if (!(await outcome.completion)) return false;
    if (this.manager.getState().focusedId === windowId) this.pushRoute(route);
    return true;
  }

  /** Close: successor focus follows ADR-002 (next-top window, else desktop). */
  async userClose(windowId: string): Promise<boolean> {
    const state = this.manager.getState();
    if (!state.windows[windowId]) return false;
    this.nextUserIntentToken += 1;
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
    this.nextUserIntentToken += 1;
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
    this.nextUserIntentToken += 1;
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
   * window on the active desktop or leaves the desktop as-is (first run stays
   * empty, US-001.EC-1).
   */
  private queueRouteReconciliation(route: OsWindowRoute): void {
    const current = this.routeReconciliation;
    const desktopDefaultView = carriesDesktopDefaultViewIntent(
      route,
      this.desktopDefaultViewIntentPending,
      current
    );
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
    const intent = this.createdSessionIntent;
    const matches = intent?.instanceKey === resolved.instanceKey;
    if (matches || (intent && pending.token > intent.afterToken)) this.createdSessionIntent = null;
    const createdSession = pending.createdSession === true || matches;
    const tracked = createdSession ? { ...pending, createdSession } : pending;
    const intentToken = this.nextUserIntentToken;
    const outcome = decideRouteReconciliation(this.manager, {
      resolved,
      route,
      desktopDefaultView,
      mode,
      createdSession,
      isCurrent: () => intentToken === this.nextUserIntentToken,
    });
    if (outcome === null) {
      this.routeReconciliation = null;
      return;
    }
    this.startRouteReconciliation(tracked, outcome);
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
