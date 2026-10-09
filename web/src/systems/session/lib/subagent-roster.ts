// The parent session's subagent roster as the session stream reports it
// (`_spec.md` API Endpoints › Session stream): `subagents_snapshot` replaces
// every row after (re)subscribe; `subagent_updated` upserts one row by id and
// never moves a row backwards in time. Between a reconnect and its snapshot the
// rows the client holds are unconfirmed (stale): drawn, but never ticking.

import type { SubagentView } from "../components/subagents/types";
import { isSubagentLive } from "../components/subagents/subagent-format";
import type { SubagentsSnapshotPayload, SubagentUpdatedPayload } from "../types";
import { subagentViewFromPayload } from "./subagent-payload";

export interface SubagentRoster {
  /** Newest first, as the snapshot orders them; rows first seen in an update lead. */
  rows: readonly SubagentView[];
  /** Rows held across a reconnect that no frame has confirmed since. */
  staleIds: ReadonlySet<string>;
}

export const EMPTY_SUBAGENT_ROSTER: SubagentRoster = { rows: [], staleIds: new Set() };

export function applySubagentsSnapshot(payload: SubagentsSnapshotPayload): SubagentRoster {
  return { rows: (payload.subagents ?? []).map(subagentViewFromPayload), staleIds: new Set() };
}

function instant(value: string): number {
  const ms = Date.parse(value);
  return Number.isFinite(ms) ? ms : Number.NEGATIVE_INFINITY;
}

export function applySubagentUpdated(
  roster: SubagentRoster,
  payload: SubagentUpdatedPayload
): SubagentRoster {
  const next = subagentViewFromPayload(payload.subagent);
  const index = roster.rows.findIndex(row => row.id === next.id);
  const staleIds = roster.staleIds.has(next.id)
    ? new Set([...roster.staleIds].filter(id => id !== next.id))
    : roster.staleIds;
  if (index < 0) return { rows: [next, ...roster.rows], staleIds };
  const current = roster.rows[index]!;
  if (instant(next.updated_at) < instant(current.updated_at)) return roster;
  const rows = [...roster.rows];
  rows[index] = { ...next, created_at: current.created_at };
  return { rows, staleIds };
}

/** A reconnect began: every held row is unconfirmed until the next snapshot. */
export function markSubagentRosterStale(roster: SubagentRoster): SubagentRoster {
  if (roster.rows.length === 0) return roster;
  return { rows: roster.rows, staleIds: new Set(roster.rows.map(row => row.id)) };
}

export function findSubagent(roster: SubagentRoster, id: string): SubagentView | undefined {
  return roster.rows.find(row => row.id === id);
}

export function liveSubagentIds(roster: SubagentRoster): ReadonlySet<string> {
  return new Set(roster.rows.filter(row => isSubagentLive(row.status)).map(row => row.id));
}
