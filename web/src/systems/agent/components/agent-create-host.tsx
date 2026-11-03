import type { ReactNode } from "react";

import {
  AgentCreateHostContext,
  type AgentCreateHostValue,
} from "@/systems/agent/lib/agent-create-host-context";

export type { AgentCreateHostValue };

export interface AgentCreateHostProviderProps {
  openDialog: AgentCreateHostValue["openDialog"];
  openForDuplicate: AgentCreateHostValue["openForDuplicate"];
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
