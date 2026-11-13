/** Enough for a burst of authoritative focus changes and their redirects; older ones are settled. */
const WRITTEN_ENTRY_LIMIT = 32;

/**
 * What happened to the router's history, by the entry key the history itself assigns
 * (`__TSR_key`, a fresh key for every push and replace, restored with its entry on Back and
 * Forward). The router bridge tells the coordinator's own writes from every other navigation.
 */
export type RouterEntryEvent =
  /** The coordinator's own push or replace committed this entry. */
  | { kind: "write"; key: string }
  /** Any other push or replace: a link, a typed URL, or the router's redirect or normalization. */
  | { kind: "push" | "replace"; key: string }
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
 * - A replace the router makes while it is still loading an owned entry (a loader redirect, a
 *   search normalization) creates an owned entry too: it is the same navigation, and every
 *   report it produces is an echo.
 * - A traversal is the operator's: it lands on an entry whose report is intent, even when the
 *   coordinator wrote that entry earlier. Nothing written before it can still be reported (the
 *   router renders the entry it landed on), so it ends every echo.
 *
 * The map lives in memory: after a reload every entry reads as intent.
 */
export class RoutingEntryLedger {
  private current: string | null = null;
  private readonly written = new Set<string>();
  /** Owned entries the router has not finished loading: a replace now is their redirect. */
  private readonly loading = new Set<string>();

  observe(event: RouterEntryEvent): void {
    switch (event.kind) {
      case "write":
        this.own(event.key);
        break;
      case "replace":
        if (this.current !== null && this.loading.has(this.current)) this.own(event.key);
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

  private own(key: string): void {
    this.written.add(key);
    this.loading.add(key);
    if (this.written.size <= WRITTEN_ENTRY_LIMIT) return;
    const oldest = this.written.values().next().value;
    if (oldest === undefined) return;
    this.written.delete(oldest);
    this.loading.delete(oldest);
  }
}
