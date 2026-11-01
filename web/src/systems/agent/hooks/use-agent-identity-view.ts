import { use } from "react";

import { resolveAgentIdentity } from "../lib/agent-identity";
import {
  AgentIdentityCatalogContext,
  type AgentIdentityScope,
} from "../lib/agent-identity-context";

/** Resolves many names at once: toasts, palette rows, and copy built outside a row component. */
export function useAgentIdentityResolverView(scope: AgentIdentityScope = {}) {
  const useCatalog = use(AgentIdentityCatalogContext);
  const agents = useCatalog(scope);
  return (name: string) => resolveAgentIdentity(name, agents);
}

export function useAgentIdentityView(name: string, scope: AgentIdentityScope = {}) {
  const useCatalog = use(AgentIdentityCatalogContext);
  return resolveAgentIdentity(name, useCatalog(scope));
}
