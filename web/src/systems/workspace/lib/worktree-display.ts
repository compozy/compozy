import type {
  DiscoveredWorktreePayload,
  WorktreeDisplayState,
  WorktreePayload,
  WorktreesResponse,
} from "../types";

/** Terminal records the daemon keeps for audit but no surface lists. */
const HIDDEN_STATES = new Set(["removed", "dismissed"]);

/**
 * One row model for both adopted records and git-discovered checkouts, so every
 * surface renders the same vocabulary from one projection.
 */
export interface WorktreeNestEntry {
  key: string;
  kind: "worktree" | "discovered";
  displayState: WorktreeDisplayState;
  name: string;
  branch: string;
  path: string;
  /** Sort input. `null` means unknown, which always sorts last. */
  activityAt: string | null;
  selectable: boolean;
  /** Functional label for a row that cannot be selected. `null` when selectable. */
  inertReason: string | null;
  /** Only discovered rows advertise the adopt affordance. */
  adoptable: boolean;
  worktree: WorktreePayload | null;
  discovered: DiscoveredWorktreePayload | null;
}

/**
 * Maps the wire's lifecycle `state` onto the presentational vocabulary.
 * `error` is the presentation of a status read failure, which the wire keeps
 * distinct from the `failed` lifecycle state (TechSpec N-004) — merging them
 * would report a rolled-back creation as an unreadable checkout.
 */
export function toWorktreeDisplayState(
  worktree: Pick<WorktreePayload, "state">,
  statusReadFailed = false
): WorktreeDisplayState {
  if (statusReadFailed) return "error";
  switch (worktree.state) {
    case "ready":
      return "ready";
    case "pending":
    case "removing":
      return "pending";
    case "missing":
      return "missing";
    case "failed":
      return "failed";
    default:
      return "error";
  }
}

const INERT_REASONS: Partial<Record<WorktreeDisplayState, string>> = {
  pending: "materializing",
  missing: "directory missing",
  failed: "creation failed",
  error: "status read failed",
};

function worktreeEntry(worktree: WorktreePayload): WorktreeNestEntry {
  const displayState = toWorktreeDisplayState(worktree);
  const inertReason = INERT_REASONS[displayState] ?? null;
  return {
    key: worktree.id,
    kind: "worktree",
    displayState,
    name: worktree.name,
    branch: worktree.branch,
    path: worktree.path,
    activityAt: worktree.updated_at,
    selectable: inertReason === null,
    inertReason,
    adoptable: false,
    worktree,
    discovered: null,
  };
}

function discoveredEntry(discovered: DiscoveredWorktreePayload): WorktreeNestEntry {
  // Git reports prunable/absent checkouts; they are listed truthfully but cannot
  // be adopted, so they carry git's own reason rather than an invented one.
  const selectable = discovered.selectable && !discovered.unavailable;
  return {
    key: `discovered:${discovered.path}`,
    kind: "discovered",
    displayState: "discovered",
    name: discovered.name,
    branch: discovered.branch,
    path: discovered.path,
    activityAt: null,
    selectable,
    inertReason: selectable ? null : discovered.reason?.trim() || "unavailable",
    adoptable: selectable,
    worktree: null,
    discovered,
  };
}

export function toWorktreeNestEntries(
  listing: Pick<WorktreesResponse, "worktrees" | "discovered"> | undefined
): WorktreeNestEntry[] {
  if (!listing) return [];
  const entries: WorktreeNestEntry[] = [];
  for (const worktree of listing.worktrees) {
    if (HIDDEN_STATES.has(worktree.state)) continue;
    entries.push(worktreeEntry(worktree));
  }
  for (const discovered of listing.discovered) {
    entries.push(discoveredEntry(discovered));
  }
  return entries;
}
