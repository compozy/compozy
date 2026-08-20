import { knowledgeMemoryKey, type KnowledgeMemoryItem } from "@/systems/knowledge";

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
