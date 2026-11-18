import type { RouterEvents, RouterHistory } from "@tanstack/react-router";

import type { OsWindowRoute } from "./os-types";
import type { OsRouterPort } from "./routing-coordinator-contract";
import type { RouterEntryEvent, RoutingEntryLedger } from "./routing-entry-ledger";

/** The part of the router's history the bridge writes to; the router's own history satisfies it. */
export interface RouterEntryHistory {
  push(path: string, state: undefined, options: { ignoreBlocker: boolean }): void;
  replace(path: string, state: undefined, options: { ignoreBlocker: boolean }): void;
}

/**
 * The coordinator's history writes, and the classification of every history notification into
 * the coordinator's own writes and everything else.
 *
 * A notification is the coordinator's own only when it arrives inside the bridge's own `push` or
 * `replace` call. The coordinator's writes are projections of authoritative window-manager state
 * and never answer a navigation blocker (`OsRouterPort`), so the history commits and notifies
 * each one synchronously inside the call. Nothing is remembered once the call returns, so nothing
 * accumulates and no later entry can be claimed by an earlier write.
 *
 * An operator navigation the coordinator writes for a control (`origin: "operator"`) answers
 * blockers like any router navigation. Held by one, it lands outside the call, if at all, and
 * reads as the operator's navigation it is.
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
  /** Set inside a history `replace` call that bypasses blockers: the router's continuation. */
  private continuing = false;
  readonly port: OsRouterPort;

  constructor(history: RouterEntryHistory, hrefOf: (route: OsWindowRoute) => string) {
    this.history = history;
    this.hrefOf = hrefOf;
    this.port = {
      navigate: (route, origin) => this.write("push", route, origin === "operator"),
      replace: route => this.write("replace", route, false),
    };
  }

  /**
   * Runs one history `replace` call. A replace that bypasses navigation blockers is committed and
   * notified inside the call (the history awaits nothing without blockers), so its notification
   * is read as the router's continuation of the navigation it is loading: router-core commits
   * every redirect it follows that way. Nothing is remembered once the call returns.
   */
  replacing<T>(bypassesBlockers: boolean, call: () => T): T {
    const outer = this.continuing;
    this.continuing = bypassesBlockers;
    try {
      return call();
    } finally {
      this.continuing = outer;
    }
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
    if (mode === "push") return { kind: "push", key };
    return { kind: "replace", key, pathname, continuation: this.continuing };
  }

  private write(mode: "push" | "replace", route: OsWindowRoute, blockable: boolean): void {
    const href = this.hrefOf(route);
    const options = { ignoreBlocker: !blockable };
    this.writing = mode;
    try {
      if (mode === "push") this.history.push(href, undefined, options);
      else this.history.replace(href, undefined, options);
    } finally {
      this.writing = null;
    }
  }
}

/** What `connectRouterEntries` reads from the router: its history and its finished loads. */
export interface RouterEntrySource {
  history: Pick<RouterHistory, "subscribe" | "replace">;
  subscribe(
    eventType: "onResolved",
    listener: (event: RouterEvents["onResolved"]) => void
  ): () => void;
}

/**
 * Feeds the ledger every history change as it happens (the history notifies inside push, replace
 * and on Back or Forward, before the router loads and reports the route) and every load the
 * router finishes. The history's `replace` runs through the bridge, which reads whether the call
 * bypasses blockers (`RouterEntryBridge.replacing`). Returns the disconnect, which restores it.
 */
export function connectRouterEntries(
  router: RouterEntrySource,
  bridge: RouterEntryBridge,
  entries: RoutingEntryLedger
): () => void {
  const history = router.history;
  const replace = history.replace;
  history.replace = (path, state, navigateOpts) =>
    bridge.replacing(navigateOpts?.ignoreBlocker === true, () =>
      replace.call(history, path, state, navigateOpts)
    );
  const stopHistory = router.history.subscribe(({ location, action }) => {
    const event = bridge.classify(action.type, location.pathname, location.state.__TSR_key);
    if (event !== null) entries.observe(event);
  });
  const stopResolved = router.subscribe("onResolved", ({ toLocation }) => {
    const key = toLocation.state.__TSR_key;
    if (key !== undefined) entries.observe({ kind: "resolved", key });
  });
  return () => {
    history.replace = replace;
    stopHistory();
    stopResolved();
  };
}
