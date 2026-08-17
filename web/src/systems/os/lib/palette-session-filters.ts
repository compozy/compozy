/**
 * The palette Sessions view's state chips — Herdr's navigator filters in
 * palette form.
 *
 * Every predicate reads the one badge dictionary through `attentionBand`, so a
 * chip can never disagree with the mark rendered on its own rows. The Idle chip
 * is the one that does not simply take a band: `attentionBand`'s fourth band is
 * `rest`, which also holds `hung`, `unhealthy`, `stopped` and `unknown`.
 * Labelling that "Idle" would be a lie about four states, so Idle means exactly
 * the `idle` badge and everything else in the band stays reachable under All —
 * the same deal Herdr's `i` filter makes.
 */
import { getSessionDisplayTitle, toSessionBadge, type SessionPayload } from "@/systems/session";

import { attentionBand } from "./attention-order";

export const PALETTE_SESSION_FILTER_IDS = [
  "all",
  "needs-you",
  "working",
  "finished",
  "idle",
] as const;

export type PaletteSessionFilterId = (typeof PALETTE_SESSION_FILTER_IDS)[number];

export interface PaletteSessionFilter {
  readonly id: PaletteSessionFilterId;
  readonly label: string;
  /** Names the filter in the empty state, so a zero match says what it means. */
  readonly emptyMessage: string;
  matches(session: SessionPayload): boolean;
}

export const PALETTE_SESSION_FILTERS = {
  all: {
    id: "all",
    label: "All",
    emptyMessage: "No sessions in this workspace yet.",
    matches: () => true,
  },
  "needs-you": {
    id: "needs-you",
    label: "Needs you",
    emptyMessage: "No session needs you right now.",
    matches: session => attentionBand(session.badge) === "needs-you",
  },
  working: {
    id: "working",
    label: "Working",
    emptyMessage: "Nothing is running right now.",
    matches: session => attentionBand(session.badge) === "working",
  },
  finished: {
    id: "finished",
    label: "Finished",
    emptyMessage: "Nothing finished right now.",
    matches: session => attentionBand(session.badge) === "finished",
  },
  idle: {
    id: "idle",
    label: "Idle",
    emptyMessage: "No idle sessions right now.",
    matches: session => toSessionBadge(session.badge) === "idle",
  },
} as const satisfies Record<PaletteSessionFilterId, PaletteSessionFilter>;

/** Chip order, left to right. */
export const PALETTE_SESSION_FILTER_LIST: readonly PaletteSessionFilter[] =
  PALETTE_SESSION_FILTER_IDS.map(id => PALETTE_SESSION_FILTERS[id]);

export function paletteSessionFilter(id: PaletteSessionFilterId): PaletteSessionFilter {
  return PALETTE_SESSION_FILTERS[id];
}

export type PaletteSessionFilterCounts = Record<PaletteSessionFilterId, number>;

/**
 * Chip counts over the whole scope, never over the rendered page — a count that
 * only described what fits on screen would send the operator looking for
 * sessions the chip already promised.
 */
export function paletteSessionFilterCounts(
  sessions: readonly SessionPayload[]
): PaletteSessionFilterCounts {
  const counts = { all: 0, "needs-you": 0, working: 0, finished: 0, idle: 0 };
  for (const session of sessions) {
    for (const filter of PALETTE_SESSION_FILTER_LIST) {
      if (filter.matches(session)) counts[filter.id] += 1;
    }
  }
  return counts;
}

/** Typing narrows by what the row shows: its title and its agent. */
export function matchesPaletteSessionQuery(session: SessionPayload, query: string): boolean {
  const needle = query.trim().toLowerCase();
  if (needle === "") return true;
  const title = getSessionDisplayTitle(session).toLowerCase();
  const agent = (session.agent_name ?? "").toLowerCase();
  return title.includes(needle) || agent.includes(needle);
}

export function filterPaletteSessions(
  sessions: readonly SessionPayload[],
  filterId: PaletteSessionFilterId,
  query: string
): SessionPayload[] {
  const filter = paletteSessionFilter(filterId);
  return sessions.filter(
    session => filter.matches(session) && matchesPaletteSessionQuery(session, query)
  );
}
