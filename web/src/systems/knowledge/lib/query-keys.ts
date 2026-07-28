import { knowledgeListStableFilter } from "@/systems/knowledge/lib/memory-list-query";
import type {
  KnowledgeListFilter,
  KnowledgeSelector,
  ListMemoryDecisionsParams,
} from "@/systems/knowledge/types";

function selectorTuple(selector?: KnowledgeSelector) {
  return [
    selector?.scope ?? "",
    selector?.workspaceId ?? "",
    selector?.agentName ?? "",
    selector?.agentTier ?? "",
  ] as const;
}

export const knowledgeKeys = {
  all: ["knowledge"] as const,
  lists: () => [...knowledgeKeys.all, "list"] as const,
  list: (filters?: KnowledgeListFilter) => {
    const normalized = knowledgeListStableFilter(filters);
    return [
      ...knowledgeKeys.lists(),
      ...selectorTuple(normalized),
      normalized.type ?? "",
      normalized.sort ?? "",
      normalized.includeSystem ?? null,
      normalized.limit ?? null,
    ] as const;
  },
  details: () => [...knowledgeKeys.all, "detail"] as const,
  detail: (filename: string, selector?: KnowledgeSelector) =>
    [...knowledgeKeys.details(), filename, ...selectorTuple(selector)] as const,
  searches: () => [...knowledgeKeys.all, "search"] as const,
  search: (
    queryText: string,
    selector?: KnowledgeSelector,
    options?: { topK?: number; includeSystem?: boolean; explain?: boolean }
  ) =>
    [
      ...knowledgeKeys.searches(),
      queryText,
      ...selectorTuple(selector),
      options?.topK ?? null,
      options?.includeSystem ?? null,
      options?.explain ?? null,
    ] as const,
  decisions: () => [...knowledgeKeys.all, "decisions"] as const,
  decisionsFor: (params?: ListMemoryDecisionsParams) =>
    [
      ...knowledgeKeys.decisions(),
      ...selectorTuple(params),
      params?.filename ?? "",
      params?.op ?? "",
      params?.since ?? "",
      params?.limit ?? null,
    ] as const,
};
