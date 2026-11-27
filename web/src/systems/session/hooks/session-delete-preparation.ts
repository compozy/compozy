import type { QueryClient } from "@tanstack/react-query";

import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";

import { readCurrentSessions } from "../lib/session-batch";
import { sessionsRunningProcesses } from "../lib/session-processes";
import { sessionComputerOf } from "../lib/session-row-ref";
import { sessionSidesRawOptions } from "../lib/session-side-query-options";
import {
  readSessionTrees,
  sessionChildrenScope,
  type SessionChildrenScope,
  type SessionTrees,
} from "../lib/session-tree";
import type { SessionPayload } from "../types";

export interface PreparedSessionDeleteTarget {
  sessions: readonly SessionPayload[];
  trees: SessionTrees;
  treeErrors: ReadonlyMap<string, unknown>;
  childrenScope: SessionChildrenScope;
  sideChats: boolean;
  processCount: number;
}

/** Resolve every value the confirmation presents before exposing it to the operator. */
export async function prepareSessionDeleteTarget(
  queryClient: QueryClient,
  sessions: readonly SessionPayload[],
  bulk: boolean,
  computerId: ComputerId
): Promise<PreparedSessionDeleteTarget> {
  const catalogRows = Object.freeze([...sessions]);
  const [currentRead, treeRead, sideChats] = await Promise.all([
    readCurrentSessions(queryClient, catalogRows, bulk).catch(() => null),
    readSessionTrees(queryClient, catalogRows),
    catalogRows.length === 1 && catalogRows[0]!.id.trim() !== ""
      ? queryClient
          .fetchQuery(
            sessionSidesRawOptions(catalogRows[0]!.workspace_id ?? "", catalogRows[0]!.id, {
              archived: true,
              computerId: isRemoteComputerId(sessionComputerOf(catalogRows[0]!))
                ? sessionComputerOf(catalogRows[0]!)
                : computerId,
            })
          )
          .then(response => response.sides.length > 0)
          .catch(() => false)
      : Promise.resolve(false),
  ]);

  const currentById = new Map((currentRead?.sessions ?? []).map(session => [session.id, session]));
  const resolved = Object.freeze(
    catalogRows.map(session => currentById.get(session.id) ?? session)
  );
  return {
    sessions: resolved,
    trees: treeRead.trees,
    treeErrors: treeRead.errors,
    childrenScope: sessionChildrenScope("delete", resolved, treeRead.trees),
    sideChats,
    processCount: sessionsRunningProcesses(resolved),
  };
}
