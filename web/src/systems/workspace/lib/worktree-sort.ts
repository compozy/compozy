import type { WorktreeDisplayState } from "../types";
import type { WorktreeNestEntry } from "./worktree-display";

/** Locked nest order (DESIGN-NOTES): every surface renders this identically. */
const STATE_GROUP_ORDER: Record<WorktreeDisplayState, number> = {
  ready: 0,
  pending: 1,
  discovered: 2,
  missing: 3,
  error: 4,
  failed: 4,
};

/** `null` is unknown activity — an unparseable stamp is unknown, never epoch. */
function activityRank(entry: WorktreeNestEntry): number | null {
  if (entry.activityAt === null) return null;
  const parsed = Date.parse(entry.activityAt);
  return Number.isNaN(parsed) ? null : parsed;
}

/**
 * State group first, then last activity descending with unknown last. Pure and
 * total: equal keys fall back to name so the order never depends on input order.
 */
export function sortWorktreeNestEntries(
  entries: ReadonlyArray<WorktreeNestEntry>
): WorktreeNestEntry[] {
  return [...entries].sort((left, right) => {
    const group = STATE_GROUP_ORDER[left.displayState] - STATE_GROUP_ORDER[right.displayState];
    if (group !== 0) return group;

    const leftRank = activityRank(left);
    const rightRank = activityRank(right);
    if (leftRank === null && rightRank !== null) return 1;
    if (leftRank !== null && rightRank === null) return -1;
    if (leftRank !== null && rightRank !== null && leftRank !== rightRank) {
      return rightRank - leftRank;
    }

    return left.name.localeCompare(right.name);
  });
}

/**
 * Adopted-only count (DESIGN-NOTES locked rule): a discovered checkout has no
 * record and never enters a worktree count.
 */
export function adoptedWorktreeCount(entries: ReadonlyArray<WorktreeNestEntry>): number {
  return entries.reduce((total, entry) => (entry.kind === "worktree" ? total + 1 : total), 0);
}

/** Collapsed-parent aggregate. Zero renders nothing rather than "0 active". */
export function runningAgentCount(entries: ReadonlyArray<WorktreeNestEntry>): number {
  return entries.reduce(
    (total, entry) => (entry.worktree?.agent_activity === "running" ? total + 1 : total),
    0
  );
}
