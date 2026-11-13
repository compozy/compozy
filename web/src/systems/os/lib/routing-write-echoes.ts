import type { OsWindowRoute } from "./os-types";
import { sameOsWindowRoute } from "./window-manager-route";

/** Enough for a burst of authoritative focus changes; older replaces are long settled. */
const REPLACED_WRITE_LIMIT = 8;

/**
 * Tells the router's echo of the coordinator's own URL writes from operator navigation.
 *
 * The router reports a location once its route renders, which can be long after the write
 * (route loaders, lazy chunks). Until then authoritative focus may have moved on, and an echo
 * read as intent would focus the window that location names again. An echo is a report of:
 *
 * - the latest write the router has not reported yet; or
 * - an earlier write a later `replace` removed from history before the router reported it.
 *   Back and Forward cannot land on such an entry, so its report is the router catching up.
 *
 * Pushed entries stay in history and remain Back targets, so a superseded push is never an
 * echo. Any location the coordinator did not write (a link, Back, Forward, a deep link, a typed
 * URL) is intent and settles the record: the router reports in write order, so nothing older
 * can still arrive.
 */
export class RoutingWriteEchoes {
  private unreported: OsWindowRoute | null = null;
  private replaced: OsWindowRoute[] = [];

  wrote(route: OsWindowRoute, mode: "push" | "replace"): void {
    const previous = this.unreported;
    if (mode === "replace" && previous !== null && !sameOsWindowRoute(previous, route)) {
      this.replaced = [...this.replaced.slice(1 - REPLACED_WRITE_LIMIT), previous];
    }
    this.unreported = route;
  }

  /** A location the router already shows without a write (a held decision route). */
  reset(): void {
    this.unreported = null;
    this.replaced = [];
  }

  /** Consumes one router report; true when it echoes a write rather than naming intent. */
  isEcho(route: OsWindowRoute): boolean {
    if (this.unreported !== null && sameOsWindowRoute(this.unreported, route)) {
      this.reset();
      return true;
    }
    const index = this.replaced.findIndex(write => sameOsWindowRoute(write, route));
    if (index >= 0) {
      this.replaced = this.replaced.slice(index + 1);
      return true;
    }
    this.reset();
    return false;
  }
}
