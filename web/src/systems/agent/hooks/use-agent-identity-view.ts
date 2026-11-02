import { use } from "react";

import { resolveAgentIdentity } from "../lib/agent-identity";
import {
  AgentIdentityCatalogContext,
  AgentIdentityScopeContext,
  type AgentIdentityScope,
} from "../lib/agent-identity-context";

/** Resolves many names at once: toasts, palette rows, and copy built outside a row component. */
export function useAgentIdentityResolverView(scope: AgentIdentityScope = {}) {
  const useCatalog = use(AgentIdentityCatalogContext);
  const owner = { ...use(AgentIdentityScopeContext), ...scope };
  const agents = useCatalog(owner);
  return (name: string) => resolveAgentIdentity(name, agents);
}

export function useAgentIdentityView(name: string, scope: AgentIdentityScope = {}) {
  const useCatalog = use(AgentIdentityCatalogContext);
  return resolveAgentIdentity(name, useCatalog({ ...use(AgentIdentityScopeContext), ...scope }));
}
