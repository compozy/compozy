import {
  knowledgeMemoryKey,
  type KnowledgeMemoryItem,
  type KnowledgeScope,
} from "@/systems/knowledge";

import type { KnowledgeRouteOptions } from "./use-knowledge-route-state";

const KNOWLEDGE_SCOPES: readonly KnowledgeScope[] = ["profile", "workspace", "agent"];

function stringParam(value: unknown): string | null {
  return typeof value === "string" ? value : null;
}

/** Reads the Knowledge deep-link (memory, scope, workspace) from a window's route search. */
export function knowledgeRouteOptions(search: Record<string, unknown>): KnowledgeRouteOptions {
  const scope = KNOWLEDGE_SCOPES.find(candidate => candidate === search.scope) ?? null;
  return {
    routeMemory: stringParam(search.memory),
    routeScope: scope,
    routeWorkspaceId: stringParam(search.workspace),
  };
}

export function knowledgeKeyForRouteMemory(
  routeMemory: string | null,
  memories: readonly KnowledgeMemoryItem[]
): string | null {
  if (routeMemory === null || routeMemory === "") return null;
  const match = memories.find(memory => memory.filename === routeMemory);
  return match === undefined ? null : knowledgeMemoryKey(match);
}

export function resolveKnowledgeSelectedKey(
  routeMemory: string | null,
  selectedMemoryKey: string | null,
  memories: readonly KnowledgeMemoryItem[]
): string | null {
  const routeKey = knowledgeKeyForRouteMemory(routeMemory, memories);
  if (
    selectedMemoryKey !== null &&
    memories.some(memory => knowledgeMemoryKey(memory) === selectedMemoryKey)
  ) {
    return selectedMemoryKey;
  }
  if (routeMemory !== null && routeMemory !== "") {
    return routeKey;
  }
  const first = memories[0];
  return first === undefined ? null : knowledgeMemoryKey(first);
}
