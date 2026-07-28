import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  listMemories,
  listMemoryDecisions,
  readMemory,
  searchMemory,
} from "@/systems/knowledge/adapters/knowledge-api";
import { knowledgeKeys } from "@/systems/knowledge/lib/query-keys";
import {
  knowledgeListPageRequest,
  knowledgeListStableFilter,
} from "@/systems/knowledge/lib/memory-list-query";
import type {
  KnowledgeListFilter,
  KnowledgeSelector,
  ListMemoryDecisionsParams,
} from "@/systems/knowledge/types";

const INITIAL_CURSOR: string | undefined = undefined;

export function memoriesListOptions(filters?: KnowledgeListFilter) {
  const stableFilters = knowledgeListStableFilter(filters);
  return infiniteQueryOptions({
    queryKey: knowledgeKeys.list(stableFilters),
    queryFn: ({ pageParam, signal }) =>
      listMemories(knowledgeListPageRequest(stableFilters, pageParam), signal),
    initialPageParam: INITIAL_CURSOR,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    staleTime: 30_000,
  });
}

export function memoryDetailOptions(selector: KnowledgeSelector | undefined, filename?: string) {
  const enabled = Boolean(selector?.scope && filename);
  return queryOptions({
    queryKey: knowledgeKeys.detail(filename ?? "", selector),
    queryFn: ({ signal }) => {
      if (!selector || !filename) {
        throw new Error("Memory detail query requires both selector and filename");
      }
      return readMemory(selector, filename, signal);
    },
    staleTime: 30_000,
    enabled,
  });
}

export function memorySearchOptions(
  selector: KnowledgeSelector | undefined,
  queryText: string,
  options?: { topK?: number; includeSystem?: boolean; explain?: boolean }
) {
  const trimmed = queryText.trim();
  const enabled = trimmed.length > 0 && Boolean(selector?.scope);
  return queryOptions({
    queryKey: knowledgeKeys.search(trimmed, selector, options),
    queryFn: ({ signal }) => {
      if (!selector || trimmed.length === 0) {
        throw new Error("Memory search query requires a selector and a non-empty query");
      }
      return searchMemory(
        {
          query_text: trimmed,
          scope: selector.scope,
          workspace_id: selector.workspaceId,
          agent_name: selector.agentName,
          agent_tier: selector.agentTier,
          top_k: options?.topK,
          include_system: options?.includeSystem,
          explain: options?.explain,
        },
        signal
      );
    },
    staleTime: 15_000,
    enabled,
  });
}

export function memoryDecisionsOptions(params: ListMemoryDecisionsParams | undefined) {
  const enabled = Boolean(params?.scope);
  return queryOptions({
    queryKey: knowledgeKeys.decisionsFor(params),
    queryFn: ({ signal }) => {
      if (!params) {
        throw new Error("Memory decisions query requires a selector");
      }
      return listMemoryDecisions(params, signal);
    },
    staleTime: 15_000,
    enabled,
  });
}
