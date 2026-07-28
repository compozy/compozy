import type { InfiniteData } from "@tanstack/react-query";

import type { KnowledgeListFilter, MemoryHeader, MemoryListPage } from "@/systems/knowledge/types";

export const DEFAULT_MEMORY_LIST_LIMIT = 50;

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

export function normalizeKnowledgeListFilter(
  filters: KnowledgeListFilter | undefined
): KnowledgeListFilter {
  return {
    scope: filters?.scope ?? "global",
    workspaceId: normalizeOptionalText(filters?.workspaceId),
    agentName: normalizeOptionalText(filters?.agentName),
    agentTier: filters?.agentTier,
    type: filters?.type,
    sort: filters?.sort,
    cursor: normalizeOptionalText(filters?.cursor),
    limit: filters?.limit,
    includeSystem: filters?.includeSystem,
  };
}

export function knowledgeListStableFilter(
  filters: KnowledgeListFilter | undefined
): Omit<KnowledgeListFilter, "cursor"> {
  const normalized = normalizeKnowledgeListFilter(filters);
  return {
    scope: normalized.scope,
    workspaceId: normalized.workspaceId,
    agentName: normalized.agentName,
    agentTier: normalized.agentTier,
    type: normalized.type,
    sort: normalized.sort,
    limit: normalized.limit,
    includeSystem: normalized.includeSystem,
  };
}

export function knowledgeListPageRequest(
  filters: Omit<KnowledgeListFilter, "cursor">,
  cursor?: string
): KnowledgeListFilter {
  return {
    ...filters,
    cursor: normalizeOptionalText(cursor),
  };
}

export interface MemoryListReadModel {
  memories: MemoryHeader[];
  total: number;
}

export function readMemoryListData(
  data: InfiniteData<MemoryListPage, unknown> | undefined
): MemoryListReadModel {
  const memories: MemoryHeader[] = [];
  const seen = new Set<string>();

  for (const page of data?.pages ?? []) {
    for (const memory of page.memories) {
      const identity = JSON.stringify([
        memory.scope,
        memory.workspace_id ?? "",
        memory.agent_name ?? "",
        memory.agent_tier ?? "",
        memory.filename,
      ]);
      if (seen.has(identity)) {
        continue;
      }
      seen.add(identity);
      memories.push(memory);
    }
  }

  return {
    memories,
    total: data?.pages[0]?.page.total ?? 0,
  };
}
