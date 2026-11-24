import { createContext } from "react";

import type {
  ComputerIconKind,
  ComputerPlatform,
} from "@/systems/computers/mocks/remote-computers-types";

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
