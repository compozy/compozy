/** Enough for a burst of authoritative focus changes; older replaces are long settled. */
const REPLACED_WRITE_LIMIT = 8;

/**
 * The history-state field that carries the id of the coordinator write that created the entry.
 * The router keeps each entry's own state, so Back or Forward to an entry reports that entry's id,
 * and an entry the coordinator did not write (a link, a typed or deep-linked URL) carries none.
 */
export const ROUTE_WRITE_STATE_KEY = "compozyRouteWrite";

/** The write id a reported location's history state carries, if the coordinator wrote it. */
export function routeWriteIdOf(state: unknown): number | null {
  if (typeof state !== "object" || state === null) return null;
  const id = (state as Record<string, unknown>)[ROUTE_WRITE_STATE_KEY];
  return typeof id === "number" ? id : null;
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
  private nextId = 0;
  /**
   * Unreported writes, oldest first: the last created the current entry, the others created
   * entries a later `replace` removed from history.
   */
  private pending: number[] = [];

  /** Records a write and returns the id its history entry carries. */
  wrote(mode: "push" | "replace"): number {
    this.nextId += 1;
    // A push leaves the current entry in history as a Back target; a replace removes it.
    const gone = mode === "replace" ? this.pending : this.pending.slice(0, -1);
    this.pending = [...gone.slice(1 - REPLACED_WRITE_LIMIT), this.nextId];
    return this.nextId;
  }

  /** A location the router already shows without a write (a held decision route). */
  reset(): void {
    this.pending = [];
  }

  /** Consumes one router report; true when its entry was created by a pending write. */
  isEcho(writeId: number | null): boolean {
    const index = writeId === null ? -1 : this.pending.indexOf(writeId);
    if (index < 0) {
      this.reset();
      return false;
    }
    this.pending = this.pending.slice(index + 1);
    return true;
  }
}
