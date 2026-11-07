import type { SessionListFilters } from "../types";

/**
 * What a session list shows right now: the active catalog, the archive, or the
 * sessions parked in Later. One value, so the archive and Later views are
 * mutually exclusive by construction.
 */
export type SessionListShelf = "active" | "archive" | "later";

/**
 * The catalog population a shelf reads; the daemon hides parked and archived rows by default.
 * `showAutomation` is the window's "Show automation runs" (#1355).
 */
export function sessionListShelfFilters(
  shelf: SessionListShelf,
  showAutomation = false
): Pick<SessionListFilters, "archive" | "exclude_automation" | "exclude_instance_main" | "later"> {
  // An always-on agent's main session lives in its avatar, never in a row.
  const population = { exclude_instance_main: true };
  if (shelf === "archive") return { ...population, archive: "only" };
  if (shelf === "later") return { ...population, later: "only" };
  // Automation runs live in their Automations run view and in search unless the window asks.
  return showAutomation ? population : { ...population, exclude_automation: true };
}

/** The noun an empty shelf names: "No sessions in Later yet." */
export function sessionListShelfNoun(shelf: SessionListShelf): string {
  if (shelf === "archive") return "archived sessions";
  if (shelf === "later") return "sessions in Later";
  return "sessions";
}
