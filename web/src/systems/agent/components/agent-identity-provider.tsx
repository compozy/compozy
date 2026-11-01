import type { ReactNode } from "react";

import { useAgentIdentityCatalog } from "../hooks/use-agent-identity";
import { AgentIdentityCatalogContext } from "../lib/agent-identity-context";

/** Gives every identity view the cached agent catalog. Mounted once, under the query client. */
export function AgentIdentityProvider({ children }: { children: ReactNode }) {
  return (
    <AgentIdentityCatalogContext value={useAgentIdentityCatalog}>
      {children}
    </AgentIdentityCatalogContext>
  );
}
