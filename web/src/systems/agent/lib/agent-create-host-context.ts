import { createContext } from "react";

import type { AgentPayload } from "../types";

export interface AgentCreateHostValue {
  openDialog: () => void;
  openForDuplicate: (agent: AgentPayload) => void;
}

export const AgentCreateHostContext = createContext<AgentCreateHostValue | null>(null);
