import type { QueryClient } from "@tanstack/react-query";

import {
  DEFAULT_MEMORY_LIST_LIMIT,
  memoriesListOptions,
  type KnowledgeListFilter,
} from "@/systems/knowledge";
import { settleRouteQueries } from "./-route-preload";

const initialKnowledgeSelector: KnowledgeListFilter = {
  scope: "global",
  includeSystem: false,
  limit: DEFAULT_MEMORY_LIST_LIMIT,
  sort: "recent",
};

export function preloadKnowledgeRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([
    queryClient.ensureInfiniteQueryData(memoriesListOptions(initialKnowledgeSelector)),
  ]);
}
