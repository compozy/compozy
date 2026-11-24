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

export interface ProjectComputerGroup<T extends MergeableSession = MergeableSession> {
  readonly key: string;
  readonly projectId: string;
  readonly projectName: string;
  readonly computerId: ComputerId;
  readonly computerName: string;
  readonly label: string;
  readonly isLocal: boolean;
  readonly offline: boolean;
  readonly platform?: string;
  readonly rows: readonly MergedSessionRow<T>[];
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
    const delta = parseTimestamp(right.session.updated_at) - parseTimestamp(left.session.updated_at);
    if (delta !== 0) return delta;
    if (left.computerId !== right.computerId) {
      return left.computerId.localeCompare(right.computerId);
    }
    return left.session.id.localeCompare(right.session.id);
  });
  return merged;
}

function oldestCursorTimestamp<T extends MergeableSession>(
  page: ComputerSessionPageInput<T>
): number {
  if (page.cursorAt !== undefined && page.cursorAt !== null) {
    return parseTimestamp(page.cursorAt);
  }
  let oldest = Number.POSITIVE_INFINITY;
  for (const session of page.sessions) {
    const ts = parseTimestamp(session.updated_at);
    if (ts < oldest) oldest = ts;
  }
  return oldest;
}

/**
 * Chooses the single computer that "Load more" should request next (UT-087):
 * among computers with `hasMore` (and not offline), picks the one whose cursor is the oldest.
 */
export function selectNextComputerToLoadMore<T extends MergeableSession>(
  pages: readonly ComputerSessionPageInput<T>[]
): ComputerId | null {
  let chosenId: ComputerId | null = null;
  let chosenCursor = Number.POSITIVE_INFINITY;
  for (const page of pages) {
    if (!page.hasMore || isComputerOffline(page.computer)) continue;
    const cursorTs = oldestCursorTimestamp(page);
    if (chosenId === null || cursorTs < chosenCursor) {
      chosenId = page.computer.id;
      chosenCursor = cursorTs;
    }
  }
  return chosenId;
}

function resolveProjectName(
  session: MergeableSession,
  workspaceNames?: ReadonlyMap<string, string>
): { id: string; name: string } {
  const rawId = session.workspace_id?.trim() ?? "";
  if (rawId !== "") {
    const mapped = workspaceNames?.get(rawId) ?? session.workspace_name?.trim();
    if (mapped) return { id: rawId, name: mapped };
    const pathTail = session.workspace_path?.split("/").filter(Boolean).at(-1);
    return { id: rawId, name: pathTail ?? rawId };
  }
  const fallbackName = session.workspace_name?.trim();
  if (fallbackName) return { id: fallbackName, name: fallbackName };
  return { id: "@user", name: "User" };
}

/**
 * Groups merged sessions by `(project, computer)` without ever merging duplicate repository names
 * across computers (UT-088, US-010.EC-2). Each group carries the label `<project> · <computer>`.
 */
export function groupSessionsByProjectAndComputer<T extends MergeableSession>(
  entries: readonly MergedSessionRow<T>[],
  workspaceNames?: ReadonlyMap<string, string>
): ProjectComputerGroup<T>[] {
  const groups = new Map<string, ProjectComputerGroup<T> & { rows: MergedSessionRow<T>[] }>();
  for (const entry of entries) {
    const project = resolveProjectName(entry.session, workspaceNames);
    const key = `${entry.computerId}::${project.id}`;
    const existing = groups.get(key);
    if (existing) {
      existing.rows.push(entry);
      continue;
    }
    groups.set(key, {
      key,
      projectId: project.id,
      projectName: project.name,
      computerId: entry.computerId,
      computerName: entry.computerName,
      label: `${project.name} · ${entry.computerName}`,
      isLocal: entry.isLocal,
      offline: entry.offline,
      platform: entry.platform,
      rows: [entry],
    });
  }
  return [...groups.values()];
}
