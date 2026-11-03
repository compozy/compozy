import { createContext } from "react";

import type { AgentPayload } from "../types";

export interface AgentCreateHostValue {
  /** With a `place`, opens on Stays on with that scope chosen. */
  openDialog: (options?: { place?: string }) => void;
  /** `profile` is the source's own profile, when the opener reads in one other than the shell's. */
  openForDuplicate: (agent: AgentPayload, options?: AgentDuplicateOpenOptions) => void;
}

export interface AgentDuplicateOpenOptions {
  profile?: string;
}

export const AgentCreateHostContext = createContext<AgentCreateHostValue | null>(null);
