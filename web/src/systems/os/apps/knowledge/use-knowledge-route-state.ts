import { useState } from "react";

import type { KnowledgeScope } from "@/systems/knowledge";

interface KnowledgeRouteOptions {
  routeMemory?: string | null;
  routeScope?: KnowledgeScope | null;
  routeWorkspaceId?: string | null;
}

/**
 * Owns the scope tab and the operator's explicit selection, resetting both when
 * the route deep-link (memory, scope, workspace) changes.
 */
function useKnowledgeRouteState(options?: KnowledgeRouteOptions) {
  const routeMemory = options?.routeMemory?.trim() || null;
  const routeScope = options?.routeScope ?? null;
  const routeWorkspaceId = options?.routeWorkspaceId?.trim() || null;

  const [activeScope, setActiveScope] = useState<KnowledgeScope>(routeScope ?? "profile");
  const [appliedRoute, setAppliedRoute] = useState({
    memory: routeMemory,
    scope: routeScope,
    workspace: routeWorkspaceId,
  });
  const [selectedMemoryKey, setSelectedMemoryKey] = useState<string | null>(null);
  if (
    routeMemory !== appliedRoute.memory ||
    routeScope !== appliedRoute.scope ||
    routeWorkspaceId !== appliedRoute.workspace
  ) {
    setAppliedRoute({ memory: routeMemory, scope: routeScope, workspace: routeWorkspaceId });
    setSelectedMemoryKey(null);
    if (routeScope !== null) {
      setActiveScope(routeScope);
    }
  }

  return {
    routeMemory,
    routeWorkspaceId,
    activeScope,
    setActiveScope,
    selectedMemoryKey,
    setSelectedMemoryKey,
  };
}

export { useKnowledgeRouteState };
export type { KnowledgeRouteOptions };
