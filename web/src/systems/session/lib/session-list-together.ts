import { LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import type { ProfileScopeParams } from "@/systems/profiles";
import type { DataScope } from "@/systems/workspace";

import type { ComputerSessionsSource } from "../hooks/use-computer-sessions-sources";
import { getSessionDisplayTitle } from "./session-display-title";
import { sessionBadgeSignal } from "./session-badge";
import type { SessionRowComputerInfo } from "./session-list-computer";
import {
  profileSessionGroups,
  projectSessionGroups,
  withInstanceProjectGroups,
  type SessionListGroupSpec,
} from "./session-list-groups";
import type { SessionListSort } from "./session-list-preferences";
import { sessionRowKey } from "./session-row-ref";
import type { SessionPayload } from "../types";

export interface TogetherFlatPageEntry {
  source: ComputerSessionsSource;
  sessions: readonly SessionPayload[];
  hasMore: boolean;
  nextCursor: string | null;
}

export interface TogetherMergedRow {
  session: SessionPayload;
  computerId: ComputerId;
  computerName: string;
  offline: boolean;
}

export interface NextTogetherComputer {
  computerId: ComputerId;
  computerName: string;
  nextCursor: string | null;
}

/**
 * Builds Together + Project groups (`project · computer`) from per-computer facets (`F6`, `N3`).
 * Groups with the same project name sit next to each other (this computer first, then registry
 * order), while User groups stay at the bottom. Never merges two computers into one group.
 */
export function buildTogetherProjectGroups(
  sources: readonly ComputerSessionsSource[],
  localLens: ProfileScopeParams
): SessionListGroupSpec[] {
  const groups: Array<{ group: SessionListGroupSpec; sourceIndex: number }> = [];
  for (let index = 0; index < sources.length; index++) {
    const source = sources[index]!;
    if (source.read !== "ok" && source.read !== "offline") continue;
    const input = {
      workspaces: source.workspaces,
      lens: source.isLocal ? localLens : { all_profiles: true as const },
      computerId: source.key,
      computerName: source.name,
      computerPlatform: source.computer.platform,
      computerIconKind: source.computer.iconKind,
      offline: source.read === "offline",
      qualifyKey: true,
    };
    const perComputer = withInstanceProjectGroups(
      projectSessionGroups(source.facets, input),
      source.residentWorkspaceIds ?? [],
      { ...input, ready: source.facets !== undefined }
    );
    for (const group of perComputer) {
      groups.push({ group, sourceIndex: index });
    }
  }
  groups.sort((left, right) => {
    const leftUser = left.group.scope.kind === "user";
    const rightUser = right.group.scope.kind === "user";
    if (leftUser !== rightUser) return leftUser ? 1 : -1;
    const leftName = (left.group.badge?.name ?? "").toLocaleLowerCase();
    const rightName = (right.group.badge?.name ?? "").toLocaleLowerCase();
    if (leftName !== rightName) return leftName.localeCompare(rightName);
    return left.sourceIndex - right.sourceIndex;
  });
  return groups.map(item => item.group);
}

/**
 * Builds Together + Profile groups (`profile · computer`) from per-computer facets (`Opus 3`, `Astra 9`).
 * Same-named profiles sit next to each other (this computer first, then registry order);
 * never merges two computers into one group.
 */
export function buildTogetherProfileGroups(
  sources: readonly ComputerSessionsSource[],
  scope: DataScope
): SessionListGroupSpec[] {
  const groups: Array<{ group: SessionListGroupSpec; sourceIndex: number }> = [];
  for (let index = 0; index < sources.length; index++) {
    const source = sources[index]!;
    if (source.read !== "ok" && source.read !== "offline") continue;
    const perComputer = profileSessionGroups(source.facets, {
      profiles: source.profiles ?? [],
      scope,
      matchedWorkspaceIds: source.matchedWorkspaceIds,
      computerId: source.key,
      computerName: source.name,
      computerPlatform: source.computer.platform,
      computerIconKind: source.computer.iconKind,
      offline: source.read === "offline",
      qualifyKey: true,
    });
    for (const group of perComputer) {
      groups.push({ group, sourceIndex: index });
    }
  }
  groups.sort((left, right) => {
    const leftName = (left.group.owner?.name ?? "").toLocaleLowerCase();
    const rightName = (right.group.owner?.name ?? "").toLocaleLowerCase();
    if (leftName !== rightName) return leftName.localeCompare(rightName);
    return left.sourceIndex - right.sourceIndex;
  });
  return groups.map(item => item.group);
}

function attentionRank(session: SessionPayload): number {
  const signal = sessionBadgeSignal(session.badge);
  if (signal.attention === "needs-you") return 0;
  if (signal.state === "failed") return 1;
  if (signal.state === "running") return 2;
  return 3;
}

function parseIso(value: string | null | undefined): number {
  if (!value) return 0;
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? 0 : parsed;
}

/**
 * Merges flat rows across computers using the window's active sort (`O1`, `O2`).
 * - `attention`: badge rank (needs-you → failed → working → idle), then `updated_at` desc.
 * - `created`: `created_at` desc.
 * - otherwise (`last_activity` / fallback): `updated_at` desc.
 */
export function mergeTogetherFlatRows(
  entries: readonly TogetherFlatPageEntry[],
  sort: SessionListSort
): TogetherMergedRow[] {
  const rows: TogetherMergedRow[] = [];
  for (const entry of entries) {
    const offline = entry.source.read === "offline";
    for (const session of entry.sessions) {
      rows.push({
        session,
        computerId: entry.source.key,
        computerName: entry.source.name,
        offline,
      });
    }
  }
  rows.sort((left, right) => compareTogetherRows(left, right, sort));
  const frontier = nextTogetherPage(entries, sort);
  if (!frontier?.tail) return rows;
  const tail = frontier.tail;
  return rows.filter(row => compareTogetherRows(row, tail, sort) <= 0);
}

function compareTogetherRows(
  left: Pick<TogetherMergedRow, "session" | "computerId">,
  right: Pick<TogetherMergedRow, "session" | "computerId">,
  sort: SessionListSort
): number {
  const rank =
    sort === "attention" ? attentionRank(left.session) - attentionRank(right.session) : 0;
  const created =
    sort === "created" ? parseIso(right.session.created_at) - parseIso(left.session.created_at) : 0;
  const name =
    sort === "name"
      ? getSessionDisplayTitle(left.session).localeCompare(
          getSessionDisplayTitle(right.session),
          undefined,
          { sensitivity: "base" }
        )
      : 0;
  return (
    rank ||
    created ||
    name ||
    parseIso(right.session.updated_at) - parseIso(left.session.updated_at) ||
    left.computerId.localeCompare(right.computerId) ||
    left.session.id.localeCompare(right.session.id)
  );
}

/** Builds a computer-qualified provenance map for Together + None flat rows (`P6`, `U5`). */
export function buildTogetherComputerMap(
  entries: readonly TogetherFlatPageEntry[]
): Map<string, SessionRowComputerInfo> {
  const map = new Map<string, SessionRowComputerInfo>();
  for (const entry of entries) {
    if (entry.source.key === LOCAL_COMPUTER_ID) continue;
    const info: SessionRowComputerInfo = {
      id: entry.source.key,
      name: entry.source.name,
      platform: entry.source.computer.platform,
      iconKind: entry.source.computer.iconKind,
      offline: entry.source.read === "offline",
      canOperate: entry.source.canOperate,
    };
    for (const session of entry.sessions) {
      map.set(sessionRowKey({ computerId: entry.source.key, sessionId: session.id }), info);
    }
  }
  return map;
}

/** Advance the source whose next page may precede the other sources' loaded tails. */
export function selectNextTogetherComputer(
  entries: readonly TogetherFlatPageEntry[],
  sort: SessionListSort = "last_activity"
): NextTogetherComputer | null {
  return nextTogetherPage(entries, sort)?.target ?? null;
}

function nextTogetherPage(entries: readonly TogetherFlatPageEntry[], sort: SessionListSort) {
  type Tail = Pick<TogetherMergedRow, "session" | "computerId">;
  let next: { target: NextTogetherComputer; tail: Tail | null } | null = null;
  for (const entry of entries) {
    if (!entry.hasMore || !entry.nextCursor || entry.source.read !== "ok") continue;
    const computerId = entry.source.key;
    const target = { computerId, computerName: entry.source.name, nextCursor: entry.nextCursor };
    const session = entry.sessions.at(-1);
    if (!session) return { target, tail: null };
    const tail = { session, computerId };
    if (!next?.tail || compareTogetherRows(tail, next.tail, sort) < 0) next = { target, tail };
  }
  return next;
}
