import { createContext } from "react";

import { mergeSessionPages, type MergedSessionRow } from "@/systems/computers/lib/merge-sessions";
import type {
  ComputerIconKind,
  ComputerPlatform,
} from "@/systems/computers/mocks/remote-computers-types";

import { buildSessionListThreads, type SessionThreadModel } from "./session-list-threads";
import type { SessionPayload } from "../types";

export interface SessionRowComputerInfo {
  id: string;
  name: string;
  platform: ComputerPlatform;
  iconKind: ComputerIconKind;
  offline: boolean;
}

/**
 * Provides remote-computer provenance (`name`, `platform`, `offline`) to session rows
 * and board cards without threading extra props through every intermediate folder/thread list.
 * Local rows are absent from the map so they render no computer label (`_uiux.md` U5).
 */
export const SessionListComputerContext = createContext<
  ReadonlyMap<string, SessionRowComputerInfo> | undefined
>(undefined);

/** Merges local thread sessions with remote fan-out rows ordered by `updated_at` desc. */
export function mergeLocalAndRemoteThreads(
  localThreads: SessionThreadModel[],
  remoteSessions: readonly MergedSessionRow<SessionPayload>[]
): SessionThreadModel[] {
  if (remoteSessions.length === 0) return localThreads;
  const grouped = new Map<
    string,
    { computer: { id: string; name: string; status: string }; sessions: SessionPayload[] }
  >();
  for (const row of remoteSessions) {
    const entry = grouped.get(row.computerId) ?? {
      computer: {
        id: row.computerId,
        name: row.computerName,
        status: row.offline ? "offline" : "connected",
      },
      sessions: [],
    };
    entry.sessions.push(row.session);
    grouped.set(row.computerId, entry);
  }
  const merged = mergeSessionPages([
    {
      computer: { id: "local", name: "This computer", status: "connected" },
      sessions: localThreads.flatMap(thread => [thread.session, ...thread.childSessions]),
    },
    ...grouped.values(),
  ]);
  return buildSessionListThreads(
    merged.map(row => row.session),
    ""
  );
}
