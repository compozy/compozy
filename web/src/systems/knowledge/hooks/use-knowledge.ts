import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  memoriesListOptions,
  memoryDecisionsOptions,
  memoryDetailOptions,
  memorySearchOptions,
} from "@/systems/knowledge/lib/query-options";
import { readMemoryListData } from "@/systems/knowledge/lib/memory-list-query";
import type {
  KnowledgeListFilter,
  KnowledgeSelector,
  ListMemoryDecisionsParams,
} from "@/systems/knowledge/types";

interface KnowledgeQueryOptions {
  enabled?: boolean;
}

export function useMemories(filters?: KnowledgeListFilter, options?: KnowledgeQueryOptions) {
  const query = useInfiniteQuery({
    ...memoriesListOptions(filters),
    enabled: (options?.enabled ?? true) && Boolean(filters?.scope),
  });
  const catalog = readMemoryListData(query.data);
  return {
    ...query,
    data: catalog.memories,
    total: catalog.total,
  };
}

export function useMemory(
  selector: KnowledgeSelector | undefined,
  filename?: string,
  options?: KnowledgeQueryOptions
) {
  return useQuery({
    ...memoryDetailOptions(selector, filename),
    enabled: (options?.enabled ?? true) && Boolean(selector?.scope && filename),
  });
}

export interface UseMemorySearchOptions extends KnowledgeQueryOptions {
  topK?: number;
  includeSystem?: boolean;
  explain?: boolean;
}

export function useMemorySearch(
  selector: KnowledgeSelector | undefined,
  queryText: string,
  options?: UseMemorySearchOptions
) {
  const trimmed = queryText.trim();
  return useQuery({
    ...memorySearchOptions(selector, trimmed, {
      topK: options?.topK,
      includeSystem: options?.includeSystem,
      explain: options?.explain,
    }),
    enabled: (options?.enabled ?? true) && trimmed.length > 0 && Boolean(selector?.scope),
  });
}

export function useMemoryDecisions(
  params: ListMemoryDecisionsParams | undefined,
  options?: KnowledgeQueryOptions
) {
  return useQuery({
    ...memoryDecisionsOptions(params),
    enabled: (options?.enabled ?? true) && Boolean(params?.scope),
  });
}
