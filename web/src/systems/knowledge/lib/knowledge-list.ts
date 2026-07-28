import { knowledgeScopeLabel } from "@/systems/knowledge/lib/knowledge-formatters";
import type { KnowledgeMemoryItem, KnowledgeScope } from "@/systems/knowledge/types";

export interface KnowledgeMemoryGroup {
  scope: KnowledgeScope;
  label: string;
  memories: KnowledgeMemoryItem[];
}

export function groupKnowledgeMemoriesByScope(
  memories: KnowledgeMemoryItem[]
): KnowledgeMemoryGroup[] {
  const buckets = new Map<KnowledgeScope, KnowledgeMemoryItem[]>();

  for (const memory of memories) {
    const scope = memory.scope;
    const list = buckets.get(scope);
    if (list) {
      list.push(memory);
      continue;
    }

    buckets.set(scope, [memory]);
  }

  return Array.from(buckets.entries()).map(([scope, memories]) => ({
    scope,
    label: knowledgeScopeLabel(scope),
    memories,
  }));
}
