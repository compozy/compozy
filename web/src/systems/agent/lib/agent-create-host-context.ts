import { createContext } from "react";

import type { AgentPayload } from "../types";

export interface AgentCreateHostValue {
  /** With a `place`, opens on Stays on with that scope chosen. */
  openDialog: (options?: { place?: string }) => void;
  openForDuplicate: (agent: AgentPayload) => void;
}

export const AgentCreateHostContext = createContext<AgentCreateHostValue | null>(null);
