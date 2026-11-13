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
 * A write adds no state to its entry: the router compares user state when a navigation targets
 * the location already shown, and an entry carrying state the navigation lacks would get a
 * duplicate entry instead of nothing. The bridge instead remembers each write's mode and href
 * until the history reports it. The history reports a write synchronously inside `push` and
 * `replace`; a navigation blocker can defer it, so a write stays expected until it is reported or
 * another navigation takes over.
 */
export class RouterEntryBridge {
  private readonly history: RouterEntryHistory;
  private readonly hrefOf: (route: OsWindowRoute) => string;
  private expected: { mode: "push" | "replace"; href: string }[] = [];
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
  classify(action: string, href: string, key: string | undefined): RouterEntryEvent | null {
    if (key === undefined) return null;
    if (action !== "PUSH" && action !== "REPLACE") {
      this.expected = [];
      return { kind: "traverse", key };
    }
    const mode = action === "PUSH" ? "push" : "replace";
    const index = this.expected.findIndex(write => write.mode === mode && write.href === href);
    if (index < 0) {
      // Another navigation took over: a write a blocker still holds reads as intent if it lands.
      this.expected = [];
      return { kind: mode, key };
    }
    this.expected = this.expected.slice(index + 1);
    return { kind: "write", key };
  }

  private write(mode: "push" | "replace", route: OsWindowRoute): void {
    const href = this.hrefOf(route);
    this.expected.push({ mode, href });
    if (mode === "push") this.history.push(href);
    else this.history.replace(href);
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
    const event = bridge.classify(action.type, location.href, location.state.__TSR_key);
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
