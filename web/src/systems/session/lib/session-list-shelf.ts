import type { SessionListFilters } from "../types";

/**
 * What a session list shows right now: the active catalog, the archive, or the
 * sessions parked in Later. One value, so the archive and Later views are
 * mutually exclusive by construction.
 */
export type SessionListShelf = "active" | "archive" | "later";

/** The catalog population a shelf reads; the daemon hides parked and archived rows by default. */
export function sessionListShelfFilters(
  shelf: SessionListShelf
): Pick<SessionListFilters, "archive" | "later"> {
  if (shelf === "archive") return { archive: "only" };
  if (shelf === "later") return { later: "only" };
  return {};
}

/** The noun an empty shelf names: "No sessions in Later yet." */
export function sessionListShelfNoun(shelf: SessionListShelf): string {
  if (shelf === "archive") return "archived sessions";
  if (shelf === "later") return "sessions in Later";
  return "sessions";
}
