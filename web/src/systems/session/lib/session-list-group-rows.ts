import { createStoreLogic } from "@xstate/store";

import type { SessionPayload } from "../types";
import type { SessionListPair } from "./session-list-groups";

/**
 * What one rendered group contributes to the selection spanning the groups — a
 * grouped list's group or one board column: its rows, in order, and its pair.
 */
export interface SessionListGroupRows {
  /** Position among the groups, as zero-padded dotted indexes ("0002.0001"); sorts as on screen. */
  rank: string;
  sessions: readonly SessionPayload[];
  visibleOrder: readonly string[];
  pair: SessionListPair | null;
  /** A read behind these rows is in flight; a session may be between two groups' reads. */
  fetching?: boolean;
}

interface SessionListGroupRowsContext {
  groups: Readonly<Record<string, SessionListGroupRows>>;
}

function sameRows(left: SessionListGroupRows | undefined, right: SessionListGroupRows): boolean {
  return (
    left !== undefined &&
    left.rank === right.rank &&
    left.pair?.workspaceId === right.pair?.workspaceId &&
    left.pair?.profile === right.pair?.profile &&
    (left.fetching ?? false) === (right.fetching ?? false) &&
    sameItems(left.sessions, right.sessions) &&
    sameItems(left.visibleOrder, right.visibleOrder)
  );
}

function sameItems<T>(left: readonly T[], right: readonly T[]): boolean {
  return left.length === right.length && left.every((item, index) => item === right[index]);
}

/**
 * The rows every expanded group of a grouped list has on screen. Each group
 * reads and pages its own catalog, so the one list-wide selection learns its
 * rows, their visible order, and the pair that owns them from here. A group
 * leaves when it collapses or unmounts.
 */
export const sessionListGroupRowsLogic = createStoreLogic({
  context: (): SessionListGroupRowsContext => ({ groups: {} }),
  on: {
    published: (context, { key, rows }: { key: string; rows: SessionListGroupRows }) =>
      sameRows(context.groups[key], rows)
        ? context
        : { groups: { ...context.groups, [key]: rows } },
    removed: (context, { key }: { key: string }) => {
      if (!(key in context.groups)) return context;
      const { [key]: _removed, ...groups } = context.groups;
      return { groups };
    },
  },
});

/** Every group's rows in on-screen order, and the pair each listed session belongs to. */
export function sessionListGroupedRows(groups: Readonly<Record<string, SessionListGroupRows>>) {
  const ordered = Object.values(groups).sort((left, right) => left.rank.localeCompare(right.rank));
  const pairs = new Map<string, SessionListPair | null>();
  for (const group of ordered) {
    for (const session of group.sessions) pairs.set(session.id, group.pair);
  }
  return {
    sessions: ordered.flatMap(group => group.sessions),
    visibleOrder: ordered.flatMap(group => group.visibleOrder),
    pairOf: (sessionId: string) => pairs.get(sessionId) ?? null,
    /** Some group is re-reading; a session missing from every group may only be moving. */
    fetching: ordered.some(group => group.fetching === true),
  };
}

/** Zero-padded dotted rank of a group at `path` (outer index first). */
export function sessionListGroupRank(path: readonly number[]): string {
  return path.map(index => String(index).padStart(4, "0")).join(".");
}

/** Stable identity of a pair for comparisons; `null` without one. */
export function sessionListPairKey(pair: SessionListPair | null): string | null {
  return pair ? `${pair.workspaceId}\u0000${pair.profile}` : null;
}
