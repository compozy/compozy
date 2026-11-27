import { LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import type { ProfileScopeParams } from "@/systems/profiles";

import type { ComputerSessionsSource } from "../hooks/use-computer-sessions-sources";
import { sessionBadgeSignal } from "./session-badge";
import type { SessionRowComputerInfo } from "./session-list-computer";
import { projectSessionGroups, type SessionListGroupSpec } from "./session-list-groups";
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
    const perComputer = projectSessionGroups(source.facets, {
      workspaces: source.workspaces,
      lens: source.isLocal ? localLens : { all_profiles: true },
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

function attentionRank(session: SessionPayload): number {
  const signal = sessionBadgeSignal(session.badge);
  if (signal.attention === "needs-you") return 0;
  if (signal.tone === "danger") return 1;
  if (signal.tone === "success") return 2;
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
  rows.sort((left, right) => {
    if (sort === "attention") {
      const rankDelta = attentionRank(left.session) - attentionRank(right.session);
      if (rankDelta !== 0) return rankDelta;
    }
    if (sort === "created") {
      const createdDelta = parseIso(right.session.created_at) - parseIso(left.session.created_at);
      if (createdDelta !== 0) return createdDelta;
    }
    const updatedDelta = parseIso(right.session.updated_at) - parseIso(left.session.updated_at);
    if (updatedDelta !== 0) return updatedDelta;
    if (left.computerId !== right.computerId) {
      return left.computerId.localeCompare(right.computerId);
    }
    return left.session.id.localeCompare(right.session.id);
  });
  return rows;
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
    };
    for (const session of entry.sessions) {
      map.set(sessionRowKey({ computerId: entry.source.key, sessionId: session.id }), info);
    }
  }
  return map;
}

/**
 * Selects which connected computer with remaining pages "Load more" advances next (`S10`):
 * picks the computer whose oldest loaded row is furthest in the past, naming it on the button.
 */
export function selectNextTogetherComputer(
  entries: readonly TogetherFlatPageEntry[]
): NextTogetherComputer | null {
  let chosen: (NextTogetherComputer & { oldestTs: number }) | null = null;
  for (const entry of entries) {
    if (!entry.hasMore || entry.source.read !== "ok") continue;
    let oldest = Number.POSITIVE_INFINITY;
    for (const session of entry.sessions) {
      const ts = parseIso(session.updated_at);
      if (ts < oldest) oldest = ts;
    }
    if (chosen === null || oldest < chosen.oldestTs) {
      chosen = {
        computerId: entry.source.key,
        computerName: entry.source.name,
        nextCursor: entry.nextCursor,
        oldestTs: oldest,
      };
    }
  }
  if (!chosen) return null;
  return {
    computerId: chosen.computerId,
    computerName: chosen.computerName,
    nextCursor: chosen.nextCursor,
  };
}
