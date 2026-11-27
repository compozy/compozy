import { LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";

/** Default first-page size requested per computer in multi-computer lists (UT-087). */
export const COMPUTER_SESSION_PAGE_LIMIT = 50;

export interface MergeSessionComputer {
  readonly id: ComputerId;
  readonly name: string;
  readonly status?: string;
  readonly platform?: string;
}

export interface MergeableSession {
  readonly id: string;
  readonly updated_at: string;
  readonly workspace_id?: string | null;
  readonly workspace_name?: string | null;
  readonly workspace_path?: string | null;
}

export interface ComputerSessionPageInput<T extends MergeableSession = MergeableSession> {
  readonly computer: MergeSessionComputer;
  readonly sessions: readonly T[];
  /** Explicit cursor timestamp (ISO or numeric ms) for paging; defaults to oldest session `updated_at`. */
  readonly cursorAt?: string | number | null;
  readonly nextCursor?: string | null;
  readonly hasMore?: boolean;
}

export interface MergedSessionRow<T extends MergeableSession = MergeableSession> {
  readonly session: T;
  readonly computerId: ComputerId;
  readonly computerName: string;
  readonly isLocal: boolean;
  readonly offline: boolean;
  readonly platform?: string;
}

function parseTimestamp(value: string | number | null | undefined): number {
  if (typeof value === "number") return Number.isFinite(value) ? value : 0;
  if (typeof value !== "string" || value.trim() === "") return 0;
  const numeric = Number(value);
  if (Number.isFinite(numeric)) return numeric;
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? 0 : parsed;
}

function isComputerOffline(computer: MergeSessionComputer): boolean {
  return computer.id !== LOCAL_COMPUTER_ID && computer.status === "offline";
}

/**
 * Merges per-computer session pages into one unified descending `updated_at` list (UT-085, UT-086).
 * Offline computers' cached rows are kept with `offline: true` and never dropped from the merged list.
 */
export function mergeSessionPages<T extends MergeableSession>(
  pages: readonly ComputerSessionPageInput<T>[]
): MergedSessionRow<T>[] {
  const merged: MergedSessionRow<T>[] = [];
  for (const page of pages) {
    const isLocal = page.computer.id === LOCAL_COMPUTER_ID;
    const offline = isComputerOffline(page.computer);
    for (const session of page.sessions) {
      merged.push({
        session,
        computerId: page.computer.id,
        computerName: page.computer.name,
        isLocal,
        offline,
        platform: page.computer.platform,
      });
    }
  }
  merged.sort((left, right) => {
    const delta =
      parseTimestamp(right.session.updated_at) - parseTimestamp(left.session.updated_at);
    if (delta !== 0) return delta;
    if (left.computerId !== right.computerId) {
      return left.computerId.localeCompare(right.computerId);
    }
    return left.session.id.localeCompare(right.session.id);
  });
  return merged;
}
