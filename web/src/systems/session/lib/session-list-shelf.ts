import { createStore } from "@xstate/store";

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

/**
 * Each window's shelf, keyed by window scope. Selecting a session remounts the window's content,
 * so a shelf held in component state fell back to the active list on every open (#1706). It lives
 * here, apart from the persisted rail state: the archive and Later stay ways of looking at the list
 * right now, so a reload, and a window that never chose, start from the active list.
 */
export const sessionListShelfStore = createStore({
  context: { byScope: {} as Record<string, SessionListShelf> },
  on: {
    shelfChanged: (context, event: { scopeId: string; shelf: SessionListShelf }) => {
      if ((context.byScope[event.scopeId] ?? "active") === event.shelf) return context;
      return { byScope: { ...context.byScope, [event.scopeId]: event.shelf } };
    },
  },
});
