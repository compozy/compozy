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
  /** Any other replace: the router's redirect or normalization, or an app's own replace. */
  | { kind: "replace"; key: string; pathname: string }
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
 * - A replace that keeps the path of an owned entry the router is still loading (a loader
 *   redirect adding a binding, a search normalization) creates an owned entry too: it is the same
 *   navigation, and every report it produces is an echo. History does not say who made a replace,
 *   and router-core marks its redirects only on private state, so the path is the line: a replace
 *   to another path is the operator's or an app's (`origin/main` read every such report as
 *   intent too), and a loader redirect to another path is read as intent with it.
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
        if (this.current !== null && this.loading.get(this.current) === event.pathname) {
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
