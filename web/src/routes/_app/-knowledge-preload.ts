import type { QueryClient } from "@tanstack/react-query";

import { settleRouteQueries } from "./-route-preload";
import {
  DEFAULT_MEMORY_LIST_LIMIT,
  type KnowledgeListFilter,
  memoriesListOptions,
} from "@/systems/knowledge";

const initialKnowledgeSelector: KnowledgeListFilter = {
  scope: "profile",
  includeSystem: false,
  limit: DEFAULT_MEMORY_LIST_LIMIT,
  sort: "recent",
};

export function preloadKnowledgeRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([
    queryClient.ensureInfiniteQueryData(memoriesListOptions(initialKnowledgeSelector)),
  ]);
}
