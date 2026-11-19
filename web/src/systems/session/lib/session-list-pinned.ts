import { createContext } from "react";

import type { SessionListFilters } from "../types";
import type { SessionListShelf } from "./session-list-shelf";

/** The Pinned read's page counts pins, each with the subtree it carries: pins past it show as "100+". */
export const SESSION_LIST_PINNED_LIMIT = 100;

/** The Pinned section's fold key among the list's group folds. */
export const SESSION_LIST_PINNED_FOLD_KEY = "pinned";

/**
 * True inside the Pinned section. A pinned row anywhere else (the archive, Later) carries a pin
 * mark instead, so its pin is visible while it waits to return.
 */
export const SessionListPinnedSectionContext = createContext(false);

/**
 * Only the active shelf has a Pinned section, so only there do the list's own reads leave pinned
 * sessions out, with every session their pins carry: a pinned session's subtree shows once, in
 * Pinned, and returns to its group and folder on unpin. The archive and Later list them in place.
 */
export function sessionListPinnedExclusion(
  shelf: SessionListShelf
): Pick<SessionListFilters, "pinned"> {
  return shelf === "active" ? { pinned: "exclude" } : {};
}

/**
 * The Pinned section has rows, is still reading them, or has other pages, so an empty list below
 * it is not (yet) an empty list: "No sessions" never flashes before the pins arrive.
 */
export function sessionListHasPinnedRows(view: {
  pinned?: { sessions: readonly unknown[]; loading?: boolean; more?: boolean };
}): boolean {
  const pinned = view.pinned;
  return Boolean(pinned?.loading || pinned?.more) || (pinned?.sessions.length ?? 0) > 0;
}

/**
 * The Pinned read: the list's own filters over pinned sessions and the subtrees they carry, in the
 * operator's Pinned order (a new pin on top).
 */
export function sessionListPinnedFilters(groupFilters: SessionListFilters): SessionListFilters {
  return {
    ...groupFilters,
    pinned: "only",
    sort: "pinned",
    limit: SESSION_LIST_PINNED_LIMIT,
  };
}
