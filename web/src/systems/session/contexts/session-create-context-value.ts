import { createContext } from "react";

export interface SessionCreateContextValue {
  openForAgent: (agentName: string) => void;
  isCreating: boolean;
  pendingAgentName: string | null;
  hasActiveWorkspace: boolean;
}

const SessionCreateContext = createContext<SessionCreateContextValue | null>(null);

export { SessionCreateContext };
