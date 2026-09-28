import {
  DEFAULT_MEMORY_LIST_LIMIT,
  type KnowledgeListFilter,
  type KnowledgeSelector,
  useMemories,
  useMemorySearch,
} from "@/systems/knowledge";

import { knowledgeSearchInfo } from "./knowledge-page-copy";
import {
  type DecorateOptions,
  decorateKnowledgeMemories,
  decorateKnowledgeSearchHits,
} from "./knowledge-page-model";

function catalogFilterFor(selector: KnowledgeSelector | null): KnowledgeListFilter | undefined {
  if (!selector) {
    return undefined;
  }
  return { ...selector, includeSystem: false, limit: DEFAULT_MEMORY_LIST_LIMIT, sort: "recent" };
}

/**
 * Reads the Knowledge list: the paged catalog, or ranked search hits while a
 * committed query is present. Errors split into a blocking one (empty list) and
 * a retry banner (rows already visible).
 */
function useKnowledgeList(
  selector: KnowledgeSelector | null,
  searchText: string,
  decorateOptions: DecorateOptions
) {
  const catalogFilter = catalogFilterFor(selector);
  const memoriesQuery = useMemories(catalogFilter, { enabled: Boolean(catalogFilter) });
  const searchActive = Boolean(selector) && searchText.length > 0;
  const searchQuery = useMemorySearch(selector ?? undefined, searchText, {
    enabled: searchActive,
  });

  const searchHits = searchQuery.data?.results ?? [];
  const memories = searchActive
    ? decorateKnowledgeSearchHits(searchHits, decorateOptions)
    : decorateKnowledgeMemories(memoriesQuery.data, decorateOptions);
  const activeQuery = searchActive ? searchQuery : memoriesQuery;
  const listError = activeQuery.error ?? null;
  const hasRows = memories.length > 0;

  const retryMemories = () => {
    if (memoriesQuery.isFetchNextPageError) {
      void memoriesQuery.fetchNextPage();
      return;
    }
    void memoriesQuery.refetch();
  };
  const retryKnowledgeList = () => {
    if (searchActive) {
      void searchQuery.refetch();
      return;
    }
    retryMemories();
  };

  return {
    memories,
    memoryCount: searchActive ? memories.length : memoriesQuery.total,
    hasMoreMemories: !searchActive && memoriesQuery.hasNextPage,
    isLoadingMoreMemories: memoriesQuery.isFetchingNextPage,
    loadMoreMemories: () => {
      void memoriesQuery.fetchNextPage();
    },
    retryMemories,
    retryKnowledgeList,
    listRetryError: hasRows ? listError : null,
    isLoading: activeQuery.isLoading && !hasRows,
    error: hasRows ? null : listError,
    searchActive,
    searchInfo: searchActive ? knowledgeSearchInfo(searchHits.length) : null,
  };
}

export { useKnowledgeList };
