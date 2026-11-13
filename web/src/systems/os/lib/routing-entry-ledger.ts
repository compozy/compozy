/** Enough for a burst of authoritative focus changes and their redirects; older ones are settled. */
const WRITTEN_ENTRY_LIMIT = 32;

/**
 * What happened to the router's history, by the entry key the history itself assigns
 * (`__TSR_key`, a fresh key for every push and replace, restored with its entry on Back and
 * Forward). The router bridge tells the coordinator's own writes from every other navigation.
 */
export type RouterEntryEvent =
  /** The coordinator's own push or replace committed this entry, at `pathname`. */
  | { kind: "write"; key: string; pathname: string }
  /** Any other push: a link, a typed URL, an app's own navigation. */
  | { kind: "push"; key: string }
  /**
   * Any other replace. `continuation`: the router committed it as the continuation of the
   * navigation it is loading (a loader or `beforeLoad` redirect). The router marks those, and only
   * those, by bypassing navigation blockers; an app's or the operator's replace answers them.
   */
  | { kind: "replace"; key: string; pathname: string; continuation: boolean }
  /** Back, Forward or a jump in history landed on this entry. */
  | { kind: "traverse"; key: string }
  /** The router finished loading and rendering this entry. */
  | { kind: "resolved"; key: string };

/**
 * Which history entries the coordinator created, so the router's report of one is read as the
 * echo of its own write, never as operator intent. Identity is the entry key, never the URL and
 * never state the coordinator adds to the entry: a link to the location already shown compares
 * equal to it, so the router commits nothing.
 *
 * - The coordinator's write creates an entry it owns.
 * - A replace creates an owned entry only when it is positively the router's continuation of an
 *   owned entry it is still loading, at the same path (a loader redirect adding a binding, a
 *   search canonicalization): the same navigation, every report of which is an echo. The mark is
 *   carried where the cause is known: router-core commits every redirect it follows through the
 *   history's public `replace` with `ignoreBlocker`, and the bridge reads that argument inside the
 *   call (`connectRouterEntries`). An app's or the operator's replace answers blockers, so it is
 *   never marked: a Tasks filter changed while the Tasks preload is still loading is intent, like
 *   any replace to another path and a redirect to another path (`origin/main` read every such
 *   report as intent).
 *
 *   An unmarked replace fails toward intent. A continuation the router stopped marking would cost
 *   a redundant reconciliation of its own window, or bring that window back when focus moved on
 *   before its report; a mark wrongly given to an app's replace would drop the operator's input.
 *   Only a navigation that bypasses blockers can be marked, and none in the app does.
 * - A traversal is the operator's: it lands on an entry whose report is intent, even when the
 *   coordinator wrote that entry earlier. Nothing written before it can still be reported (the
 *   router renders the entry it landed on), so it ends every echo.
 *
 * The map lives in memory: after a reload every entry reads as intent.
 */
export class RoutingEntryLedger {
  private current: string | null = null;
  private readonly written = new Set<string>();
  /** Owned entries the router has not finished loading, by path: a replace keeping it continues them. */
  private readonly loading = new Map<string, string>();

  observe(event: RouterEntryEvent): void {
    switch (event.kind) {
      case "write":
        this.own(event.key, event.pathname);
        break;
      case "replace":
        if (event.continuation && this.continues(event.pathname)) {
          this.own(event.key, event.pathname);
        }
        break;
      case "push":
        break;
      case "traverse":
        this.written.clear();
        this.loading.clear();
        break;
      case "resolved":
        this.loading.delete(event.key);
        return;
    }
    this.current = event.key;
  }

  /** True when the reported entry was created by the coordinator's own navigation. */
  isEcho(key: string | null): boolean {
    return key !== null && this.written.has(key);
  }

  /** True when the reported entry is the one history shows now: no later navigation replaced it. */
  isCurrent(key: string | null): boolean {
    return key !== null && key === this.current;
  }

  /** Whether the current entry is an owned one the router is still loading at `pathname`. */
  private continues(pathname: string): boolean {
    return this.current !== null && this.loading.get(this.current) === pathname;
  }

  private own(key: string, pathname: string): void {
    this.written.add(key);
    this.loading.set(key, pathname);
    if (this.written.size <= WRITTEN_ENTRY_LIMIT) return;
    const oldest = this.written.values().next().value;
    if (oldest === undefined) return;
    this.written.delete(oldest);
    this.loading.delete(oldest);
  }
}
