import type { QueryClient } from "@tanstack/react-query";

import { settleRouteQueries } from "./-route-preload";
import {
  DEFAULT_MEMORY_LIST_LIMIT,
  type KnowledgeListFilter,
  memoriesListOptions,
} from "@/systems/knowledge";
import { actingProfile, readProfileLens, readProfileView } from "@/systems/profiles";

const initialKnowledgeSelector: KnowledgeListFilter = {
  scope: "profile",
  includeSystem: false,
  limit: DEFAULT_MEMORY_LIST_LIMIT,
  sort: "recent",
};

export function preloadKnowledgeRoute(queryClient: QueryClient): Promise<void> {
  const profile = actingProfile(readProfileView(queryClient, readProfileLens()));
  return settleRouteQueries([
    queryClient.ensureInfiniteQueryData(
      memoriesListOptions({ ...initialKnowledgeSelector, profile })
    ),
  ]);
}
