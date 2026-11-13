import { resolveAppDescriptorForPath as resolveAppForPath } from "./app-catalog";
import type { OsWindowRoute, WindowManagerOpenOutcome } from "./os-types";
import type { RouteReconciliation, RoutingManager } from "./routing-coordinator-contract";
import {
  carriesDesktopDefaultViewIntent,
  withoutDesktopDefaultViewIntent,
} from "./routing-default-view-intent";
import { decideRouteReconciliation } from "./routing-route-decision";
import { sameOsWindowRoute } from "./window-manager-route";

/** What the reconciler reads from and reports back to its `RoutingCoordinator`. */
export interface RouteReconcilerHost {
  readonly manager: RoutingManager;
  /** Hydration finished and no routed decision surface holds the URL. */
  canReconcile(): boolean;
  desktopDefaultViewIntentPending(): boolean;
  /** The coordinator's newest user-intent token (rule 3 coalescing). */
  userIntentToken(): number;
  /** A reconciliation carrying the desktop default-view intent applied. */
  settleDesktopDefaultViewIntent(): void;
  reportAuthoritativeState(): void;
  /** A reconciliation this client's own navigation asked for focused `windowId` (#1655). */
  handOffKeyboard(windowId: string): void;
}

/**
 * Route-originated reconciliation (rule 2): store updates only, no history. One reconciliation
 * is pending at a time; a newer route replaces it, and a stale completion writes nothing.
 */
export class RouteReconciler {
  private readonly host: RouteReconcilerHost;
  private current: RouteReconciliation | null = null;
  private nextToken = 0;
  private pendingNavigateMode: "push" | "pop" | null = null;
  private createdSessionIntent: { instanceKey: string; afterToken: number } | null = null;

  constructor(host: RouteReconcilerHost) {
    this.host = host;
  }

  /** The reconciliation waiting on the daemon or its command fence, if any. */
  get pending(): RouteReconciliation | null {
    return this.current;
  }

  /** One-shot navigation mode the next reconciliation consumes (ADR-011). */
  noteNavigateMode(mode: "push" | "pop"): void {
    this.pendingNavigateMode = mode;
  }

  /** The route landing `sessionId` prefers an existing Session window (#825). */
  noteCreatedSession(sessionId: string): void {
    const instanceKey = sessionId.trim();
    const afterToken = this.nextToken;
    this.createdSessionIntent = instanceKey ? { instanceKey, afterToken } : null;
  }

  /**
   * The desktop URL (`/`) opens nothing — it focuses an existing dashboard window on the active
   * desktop or leaves the desktop as-is (first run stays empty, US-001.EC-1).
   */
  queue(route: OsWindowRoute, options: { keyboard?: boolean } = {}): void {
    const current = this.current;
    const keyboard = options.keyboard === true;
    const desktopDefaultView = carriesDesktopDefaultViewIntent(
      route,
      this.host.desktopDefaultViewIntentPending(),
      current
    );
    if (current && sameOsWindowRoute(current.route, route)) {
      if (keyboard && current.keyboard !== true) this.current = { ...current, keyboard };
      if (!current.inFlight) this.reconcilePending();
      return;
    }
    this.nextToken += 1;
    this.current = {
      token: this.nextToken,
      route,
      desktopDefaultView,
      inFlight: false,
      ...(keyboard ? { keyboard } : {}),
    };
    this.reconcilePending();
  }

  reconcilePending(): void {
    const pending = this.current;
    if (pending === null || pending.inFlight || !this.host.canReconcile()) return;
    const manager = this.host.manager;
    const state = manager.getState();
    if (state.client === null || state.hydration !== "live") return;
    const desktopDefaultView = pending.desktopDefaultView;
    const route = withoutDesktopDefaultViewIntent(pending.route);
    const resolved = resolveAppForPath(route.pathname);
    const mode = this.pendingNavigateMode;
    this.pendingNavigateMode = null;
    if (!resolved) {
      this.current = null;
      return;
    }
    const intent = this.createdSessionIntent;
    const matches = intent?.instanceKey === resolved.instanceKey;
    if (matches || (intent && pending.token > intent.afterToken)) this.createdSessionIntent = null;
    const createdSession = pending.createdSession === true || matches;
    const tracked = createdSession ? { ...pending, createdSession } : pending;
    const intentToken = this.host.userIntentToken();
    const outcome = decideRouteReconciliation(manager, {
      resolved,
      route,
      desktopDefaultView,
      mode,
      createdSession,
      isCurrent: () => intentToken === this.host.userIntentToken(),
    });
    if (outcome === null) {
      this.current = null;
      return;
    }
    this.start(tracked, outcome);
  }

  invalidate(): void {
    this.nextToken += 1;
    this.current = null;
    this.pendingNavigateMode = null;
  }

  private start(pending: RouteReconciliation, outcome: WindowManagerOpenOutcome): void {
    if (!outcome.accepted) {
      if (this.current?.token !== pending.token) return;
      this.current = { ...pending, inFlight: false };
      return;
    }
    this.current = { ...pending, inFlight: true };
    void outcome.completion.then(applied => {
      if (this.current?.token !== pending.token) return;
      if (!applied) {
        this.current = { ...pending, inFlight: false };
        return;
      }
      if (pending.desktopDefaultView) this.host.settleDesktopDefaultViewIntent();
      const keyboard = this.current?.keyboard === true;
      this.current = null;
      this.host.reportAuthoritativeState();
      if (keyboard) this.host.handOffKeyboard(outcome.windowId);
    });
  }
}
