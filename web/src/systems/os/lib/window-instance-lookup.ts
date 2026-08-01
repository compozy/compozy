import type { OsAppId, OsWindow } from "./os-types";

/**
 * Window IDs are opaque and generated (ADR-010): nothing derives identity from
 * them any more. Launch surfaces resolve windows semantically by
 * `(app, instanceKey)` instead, ordered by the client's own focus history.
 */
export interface WindowInstanceQuery {
  app: OsAppId;
  /** Session windows dedup on their session id; plain apps pass nothing. */
  instanceKey?: string | null;
}

/** Matches the daemon's `w-<26 hex>` generator so client and agent opens are indistinguishable. */
export function randomOsWindowId(): string {
  const bytes = new Uint8Array(13);
  globalThis.crypto.getRandomValues(bytes);
  return `w-${Array.from(bytes, byte => byte.toString(16).padStart(2, "0")).join("")}`;
}

function matchesQuery(win: OsWindow, query: WindowInstanceQuery): boolean {
  if (win.app !== query.app) return false;
  // A query without an instance key means "any instance of this app" — the
  // dock/palette launch semantics. With one it is an exact identity match.
  const wanted = query.instanceKey ?? null;
  return wanted === null || win.instanceKey === wanted;
}

/**
 * Every live instance of the query, in a stable order. Window IDs arrive sorted
 * from the daemon's map serialization, so this order survives focus changes —
 * which is what a repeatable "cycle through instances" gesture needs.
 */
export function windowInstancesFor(
  windows: Readonly<Record<string, OsWindow>>,
  query: WindowInstanceQuery
): OsWindow[] {
  return Object.values(windows)
    .filter(win => matchesQuery(win, query))
    .sort((left, right) => left.id.localeCompare(right.id));
}

/**
 * Most-recently-focused instance first. `focusOrder` is newest-first, so its
 * index doubles as recency; unvisited windows keep the stable tail order.
 */
export function mruWindowInstance(
  windows: Readonly<Record<string, OsWindow>>,
  focusOrder: readonly string[],
  query: WindowInstanceQuery
): OsWindow | null {
  const instances = windowInstancesFor(windows, query);
  if (instances.length === 0) return null;
  let best = instances[0] as OsWindow;
  let bestRank = Number.POSITIVE_INFINITY;
  for (const instance of instances) {
    const index = focusOrder.indexOf(instance.id);
    const rank = index < 0 ? Number.POSITIVE_INFINITY : index;
    if (rank < bestRank) {
      best = instance;
      bestRank = rank;
    }
  }
  return best;
}

/**
 * Launch-surface activation target: the MRU instance the first time, then the
 * next one in stable order on every repeat while that instance stays focused
 * (ADR-002 dock cycling).
 */
export function activationTarget(
  windows: Readonly<Record<string, OsWindow>>,
  focusOrder: readonly string[],
  focusedId: string | null,
  query: WindowInstanceQuery
): OsWindow | null {
  const instances = windowInstancesFor(windows, query);
  if (instances.length === 0) return null;
  const focusedIndex = instances.findIndex(instance => instance.id === focusedId);
  if (focusedIndex < 0) return mruWindowInstance(windows, focusOrder, query);
  return instances[(focusedIndex + 1) % instances.length] ?? null;
}

/** Dock aggregation (ADR-010 §3): an app lights while any instance is live. */
export type OsAppRunState = "closed" | "open" | "focused" | "minimized";

export function appRunState(
  windows: Readonly<Record<string, OsWindow>>,
  focusedId: string | null,
  app: OsAppId
): OsAppRunState {
  const instances = windowInstancesFor(windows, { app });
  if (instances.length === 0) return "closed";
  if (instances.some(instance => instance.id === focusedId && !instance.minimized))
    return "focused";
  if (instances.some(instance => !instance.minimized)) return "open";
  return "minimized";
}
