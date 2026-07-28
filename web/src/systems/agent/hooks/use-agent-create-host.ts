import { use } from "react";

import {
  AgentCreateHostContext,
  type AgentCreateHostValue,
} from "@/systems/agent/lib/agent-create-host-context";

export function useAgentCreateHost(): AgentCreateHostValue {
  const value = use(AgentCreateHostContext);
  if (!value) {
    throw new Error("useAgentCreateHost must be used within AgentCreateHostProvider");
  }
  return value;
}
