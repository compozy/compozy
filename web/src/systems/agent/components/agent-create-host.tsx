import type { ReactNode } from "react";

import {
  AgentCreateHostContext,
  type AgentCreateHostValue,
} from "@/systems/agent/lib/agent-create-host-context";
import type { AgentPayload } from "@/systems/agent/types";

export type { AgentCreateHostValue };

export interface AgentCreateHostProviderProps {
  openDialog: () => void;
  openForDuplicate: (agent: AgentPayload) => void;
  children: ReactNode;
}

export function AgentCreateHostProvider({
  openDialog,
  openForDuplicate,
  children,
}: AgentCreateHostProviderProps) {
  return (
    <AgentCreateHostContext value={{ openDialog, openForDuplicate }}>
      {children}
    </AgentCreateHostContext>
  );
}
