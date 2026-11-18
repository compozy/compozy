import type { resolveAppDescriptorForPath } from "./app-catalog";
import { createdSessionHostWindow, landCreatedSession } from "./created-session-window";
import type { OsWindowRoute, WindowManagerOpenOutcome } from "./os-types";
import type { RoutingManager } from "./routing-coordinator-contract";
import type { RoutingEntryLedger } from "./routing-entry-ledger";
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
  /** False once a newer operator choice owns focus; a created-session landing then skips it. */
  isCurrent: () => boolean;
}

/**
 * What a route-originated reconciliation (Routing Model rule 2) asks of the window manager:
 * the command to wait on, or `null` when the store already shows the route — or, for the
 * desktop URL, must not change. Never writes history.
 */
export function decideRouteReconciliation(
  manager: RoutingManager,
  { resolved, route, desktopDefaultView, mode, createdSession, isCurrent }: RouteDecisionInput
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
  if (mode === "pop" && existing && canPop) {
    return { windowId: existing.id, ...manager.popWindowRoute(existing.id, route) };
  }
  if (createdSession && app.id === "session" && instanceKey) {
    const host = createdSessionHostWindow(state, { instanceKey, route });
    if (host) return landCreatedSession(manager, host, { instanceKey, route }, isCurrent);
  }
  return manager.openOrFocus({
    app: app.id,
    instanceKey: instanceKey ?? undefined,
    route,
    ...(mode === "push" ? { navigateMode: "push" as const } : {}),
  });
}

/** The focused window's route, or the desktop when nothing is focused. */
export function focusedRouteOrDesktop(
  state: ReturnType<RoutingManager["getState"]>
): OsWindowRoute {
  const focused = state.focusedId !== null ? state.windows[state.focusedId] : null;
  return focused ? focused.route : { pathname: "/", search: {} };
}

/**
 * The newest canonical report of the coordinator's own navigation, kept while a window activation
 * settles: an activation that settles without moving focus (a repeated "Go to tab", a refused
 * competing activation) must still let the focused window adopt it. Reconsidered once none is in
 * flight, under `adoptCanonicalRoute`'s guards with the entry still the history's current one;
 * dropped when any guard fails. A newer canonical report replaces it.
 */
export class CanonicalCandidate {
  private readonly entries: Pick<RoutingEntryLedger, "isCurrent">;
  private pending: { route: OsWindowRoute; entryKey: string | null } | null = null;

  constructor(entries: Pick<RoutingEntryLedger, "isCurrent">) {
    this.entries = entries;
  }

  /** A report of an entry history no longer shows (a re-report of the write) never displaces it. */
  note(route: OsWindowRoute, entryKey: string | null): void {
    if (this.entries.isCurrent(entryKey)) this.pending = { route, entryKey };
  }

  /** Call with no activation settling. Returns the route the coordinator wrote, or the adopted one. */
  settle(
    manager: Pick<RoutingManager, "getState" | "navigateWindow">,
    written: OsWindowRoute | null
  ): OsWindowRoute | null {
    const candidate = this.pending;
    this.pending = null;
    if (candidate === null) return written;
    const current = this.entries.isCurrent(candidate.entryKey);
    return adoptCanonicalRoute(manager, written, candidate.route, current) ?? written;
  }
}

/**
 * The router's canonical form of the coordinator's own current navigation (a loader redirect that
 * adds a session's workspace binding, a search normalization) belongs to the window it shows: its
 * stored route becomes the canonical one, with no focus change and no history write, so its data
 * scope reads the binding and the next authoritative report finds the URL already right.
 *
 * Adopted only while that navigation is still the current one: `current` says the reported entry
 * is the history's current entry (`CanonicalCandidate` waits out settling activations), the focused window still holds the
 * route the coordinator wrote, and the canonical form keeps its path. Otherwise focus has moved on
 * and the canonical form is dropped; the window is canonicalized the next time it is shown.
 * Returns the adopted route, `null` when nothing was adopted or the window refused the route.
 */
export function adoptCanonicalRoute(
  manager: Pick<RoutingManager, "getState" | "navigateWindow">,
  written: OsWindowRoute | null,
  canonical: OsWindowRoute,
  current: boolean
): OsWindowRoute | null {
  if (!current || written === null || sameOsWindowRoute(written, canonical)) return null;
  const state = manager.getState();
  const focused = state.focusedId !== null ? state.windows[state.focusedId] : null;
  if (!focused || !sameOsWindowRoute(focused.route, written)) return null;
  if (canonical.pathname !== written.pathname) return null;
  // A refused command leaves the stored route the written one: keeping `currentRoute` there too
  // means the next authoritative report writes nothing instead of undoing the redirect.
  return manager.navigateWindow(focused.id, canonical).accepted ? canonical : null;
}
