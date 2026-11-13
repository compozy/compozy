import type { RouterEvents, RouterHistory } from "@tanstack/react-router";

import type { OsWindowRoute } from "./os-types";
import type { OsRouterPort } from "./routing-coordinator-contract";
import type { RouterEntryEvent, RoutingEntryLedger } from "./routing-entry-ledger";

/** The part of the router's history the bridge writes to; the router's own history satisfies it. */
export interface RouterEntryHistory {
  push(path: string): void;
  replace(path: string): void;
}

/**
 * The coordinator's history writes, and the classification of every history notification into
 * the coordinator's own writes and everything else.
 *
 * A notification is the coordinator's own only when it arrives inside the bridge's own `push` or
 * `replace` call: the history notifies synchronously inside the call when nothing defers it.
 * Nothing is remembered once the call returns, so a write a navigation blocker cancelled can never
 * claim a later entry, and nothing accumulates.
 *
 * A write a blocker deferred and later let through lands outside the call and reads as any other
 * navigation: the route the coordinator wrote is still its current route then, so reconciling it
 * finds the focused window already showing it and does nothing. A newer coordinator write while
 * the blocker still holds one is held by the same blocker, which answers only its newest request,
 * so only the newest write can land.
 *
 * A write adds no state to its entry: the router compares user state when a navigation targets
 * the location already shown, and an entry carrying state the navigation lacks would get a
 * duplicate entry instead of nothing.
 */
export class RouterEntryBridge {
  private readonly history: RouterEntryHistory;
  private readonly hrefOf: (route: OsWindowRoute) => string;
  /** The mode of the bridge's own call in progress, until its notification arrives. */
  private writing: "push" | "replace" | null = null;
  readonly port: OsRouterPort;

  constructor(history: RouterEntryHistory, hrefOf: (route: OsWindowRoute) => string) {
    this.history = history;
    this.hrefOf = hrefOf;
    this.port = {
      navigate: route => this.write("push", route),
      replace: route => this.write("replace", route),
    };
  }

  /**
   * One history notification as the ledger reads it; `null` for an entry without a key, which
   * the router's own history never produces.
   */
  classify(action: string, pathname: string, key: string | undefined): RouterEntryEvent | null {
    if (key === undefined) return null;
    if (action !== "PUSH" && action !== "REPLACE") return { kind: "traverse", key };
    const mode = action === "PUSH" ? "push" : "replace";
    if (this.writing === mode) {
      // Only the call's own commit: a navigation it triggers synchronously is not the write.
      this.writing = null;
      return { kind: "write", key, pathname };
    }
    return mode === "push" ? { kind: "push", key } : { kind: "replace", key, pathname };
  }

  private write(mode: "push" | "replace", route: OsWindowRoute): void {
    const href = this.hrefOf(route);
    this.writing = mode;
    try {
      if (mode === "push") this.history.push(href);
      else this.history.replace(href);
    } finally {
      this.writing = null;
    }
  }
}

/** What `connectRouterEntries` reads from the router: its history and its finished loads. */
export interface RouterEntrySource {
  history: Pick<RouterHistory, "subscribe">;
  subscribe(
    eventType: "onResolved",
    listener: (event: RouterEvents["onResolved"]) => void
  ): () => void;
}

/**
 * Feeds the ledger every history change as it happens (the history notifies inside push, replace
 * and on Back or Forward, before the router loads and reports the route) and every load the
 * router finishes. Returns the unsubscribe.
 */
export function connectRouterEntries(
  router: RouterEntrySource,
  bridge: RouterEntryBridge,
  entries: RoutingEntryLedger
): () => void {
  const stopHistory = router.history.subscribe(({ location, action }) => {
    const event = bridge.classify(action.type, location.pathname, location.state.__TSR_key);
    if (event !== null) entries.observe(event);
  });
  const stopResolved = router.subscribe("onResolved", ({ toLocation }) => {
    const key = toLocation.state.__TSR_key;
    if (key !== undefined) entries.observe({ kind: "resolved", key });
  });
  return () => {
    stopHistory();
    stopResolved();
  };
}
