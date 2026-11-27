import type { ComputerId } from "@/lib/computer-scope";
import type { ProfileScopeParams } from "@/systems/profiles";
import type { DataScope } from "@/systems/workspace";

import type { SessionListPair } from "../lib/session-list-groups";
import { sessionListPinnedFilters } from "../lib/session-list-pinned";
import type { SessionListShelf } from "../lib/session-list-shelf";
import type { SessionListFilters, SessionPayload } from "../types";
import { useSessionCatalog } from "./use-session-catalog";
import type { SessionListPartitions } from "./use-session-list-partitions";
import { useSessionPinnedOrder } from "./use-session-pinned-order";

/** The pins of the list's scope and filters with the subtrees they carry, in Pinned order. */
export interface SessionListPinnedRead {
  sessions: readonly SessionPayload[];
  loading: boolean;
  failed: boolean;
  /** More pins than one page of the read holds. */
  more: boolean;
  retry: () => void;
  /** The read's own paging (by pin), independent of the list's pages. */
  catalog?: Pick<
    ReturnType<typeof useSessionCatalog>,
    "sessions" | "paging" | "previous" | "next" | "failed" | "previousPage" | "nextPage" | "retry"
  >;
  partitions?: SessionListPartitions;
  folderOf?: ReadonlyMap<string, string | null>;
  /**
   * Gives the listed pins their order among the Pinned slots they hold; present where the list
   * reads one (computer, workspace, profile) pair, the scope a reorder is authorized in (`O6`).
   */
  reorder?: (sessionIds: readonly string[]) => void;
}

/**
 * The Pinned section's read: the list's own filters over pins and what they carry, in Pinned
 * order. Absent where the list has no Pinned section (the archive and Later shelves).
 */
export function useSessionListPinnedRead({
  scope,
  pair,
  groupFilters,
  enabled,
  shelf,
  pending,
  computerId,
  lens,
  matchedWorkspaceIds,
  allowReorder = true,
}: {
  scope: DataScope | null;
  pair: SessionListPair | null;
  groupFilters: SessionListFilters;
  enabled: boolean;
  shelf: SessionListShelf;
  pending: boolean;
  computerId?: ComputerId;
  lens?: ProfileScopeParams;
  matchedWorkspaceIds?: readonly string[] | null;
  allowReorder?: boolean;
}): SessionListPinnedRead | undefined {
  // Only the active shelf has a Pinned section.
  const shown = enabled && shelf === "active";
  const read = useSessionCatalog(scope, sessionListPinnedFilters(groupFilters), shown, {
    computerId,
    lens,
    matchedWorkspaceIds,
  });
  const reorder = useSessionPinnedOrder(allowReorder ? pair : null);
  if (!shown) return undefined;
  // While what the list reads under is unresolved, its pins read as loading, never as rows.
  return {
    sessions: pending ? [] : read.sessions,
    loading: read.loading || pending,
    failed: read.failed,
    more: read.next || read.previous,
    retry: read.retry,
    catalog: read,
    reorder,
  };
}
