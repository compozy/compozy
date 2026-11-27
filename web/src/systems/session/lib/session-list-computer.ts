import { createContext } from "react";

import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import type {
  ComputerIconKind,
  ComputerPlatform,
} from "@/systems/computers/mocks/remote-computers-types";

import { sessionRowKey } from "./session-row-ref";

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
 * Keys use `sessionRowKey({ computerId, sessionId })` (with a bare `sessionId` fallback for
 * prototype fixtures). Local rows are absent so they render no computer label (`_uiux.md` U5).
 */
export const SessionListComputerContext = createContext<
  ReadonlyMap<string, SessionRowComputerInfo> | undefined
>(undefined);

/** Resolves a row's computer label without colliding identical session ids across computers. */
export function lookupSessionComputer(
  map: ReadonlyMap<string, SessionRowComputerInfo> | undefined,
  sessionId: string,
  computerId?: ComputerId
): SessionRowComputerInfo | undefined {
  if (!map) return undefined;
  if (computerId !== undefined) {
    if (!isRemoteComputerId(computerId)) return undefined;
    return map.get(sessionRowKey({ computerId, sessionId })) ?? map.get(sessionId);
  }
  return map.get(sessionId);
}
