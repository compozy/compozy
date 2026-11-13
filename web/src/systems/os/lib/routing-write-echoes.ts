/** Enough for a burst of authoritative focus changes; older replaces are long settled. */
const REPLACED_WRITE_LIMIT = 8;

/**
 * The history-state field that carries the id of the coordinator write that created the entry.
 * The router keeps each entry's own state, so Back or Forward to an entry reports that entry's id,
 * and an entry the coordinator did not write (a link, a typed or deep-linked URL) carries none.
 */
export const ROUTE_WRITE_STATE_KEY = "compozyRouteWrite";

/**
 * A coordinator write's identity: a random token of the `RoutingWriteEchoes` that made it, then
 * its ordinal there. History state outlives the page, so entries written before a reload or by
 * another coordinator in the same tab carry another token and never equal a write of this one.
 */
export type RouteWriteId = string;

/** The write id a reported location's history state carries, if a coordinator wrote it. */
export function routeWriteIdOf(state: unknown): RouteWriteId | null {
  if (typeof state !== "object" || state === null) return null;
  const id = (state as Record<string, unknown>)[ROUTE_WRITE_STATE_KEY];
  return typeof id === "string" ? id : null;
}

function randomWriterToken(): string {
  const bytes = new Uint8Array(8);
  globalThis.crypto.getRandomValues(bytes);
  return Array.from(bytes, byte => byte.toString(16).padStart(2, "0")).join("");
}

/**
 * Tells the router's echo of the coordinator's own URL writes from operator navigation, by the
 * identity of the history entry the router reports, never by its URL.
 *
 * The router reports a location once its route renders, which can be long after the write
 * (route loaders, lazy chunks). Until then authoritative focus may have moved on, and an echo
 * read as intent would focus the window that location names again. An echo is a report of an
 * entry created by:
 *
 * - the latest write the router has not reported yet; or
 * - an earlier unreported write a later `replace` removed from history. Back and Forward cannot
 *   land on such an entry, so its report is the router catching up.
 *
 * A pushed entry stays in history as a Back target, so a later push ends its echo. Any other
 * entry is intent and settles the record, even when its URL equals a pending write: Back or
 * Forward to an older entry carries an older id, and a link, a typed URL or a deep link carries
 * none. The router reports in write order, so nothing older can still arrive.
 */
export class RoutingWriteEchoes {
  private readonly writer = randomWriterToken();
  private ordinal = 0;
  /**
   * Unreported writes, oldest first: the last created the current entry, the others created
   * entries a later `replace` removed from history.
   */
  private pending: RouteWriteId[] = [];

  /** Records a write and returns the id its history entry carries. */
  wrote(mode: "push" | "replace"): RouteWriteId {
    this.ordinal += 1;
    const id = `${this.writer}:${this.ordinal}`;
    // A push leaves the current entry in history as a Back target; a replace removes it.
    const gone = mode === "replace" ? this.pending : this.pending.slice(0, -1);
    this.pending = [...gone.slice(1 - REPLACED_WRITE_LIMIT), id];
    return id;
  }

  /** A location the router already shows without a write (a held decision route). */
  reset(): void {
    this.pending = [];
  }

  /** Consumes one router report; true when its entry was created by a pending write. */
  isEcho(writeId: RouteWriteId | null): boolean {
    const index = writeId === null ? -1 : this.pending.indexOf(writeId);
    if (index < 0) {
      this.reset();
      return false;
    }
    this.pending = this.pending.slice(index + 1);
    return true;
  }
}
