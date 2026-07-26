import { QueryClient } from "@tanstack/react-query";

const DEFAULT_QUERY_STALE_TIME_MS = 30_000;
const DEFAULT_QUERY_GC_TIME_MS = 10 * 60 * 1_000;
const DEFAULT_QUERY_RETRY_COUNT = 1;

export interface RouterContext {
  queryClient: QueryClient;
}

function getContext(): RouterContext {
  // Keep global TanStack behavior explicit; domain query-options extend this
  // baseline where a longer warm-cache policy is part of the product contract.
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: DEFAULT_QUERY_STALE_TIME_MS,
        gcTime: DEFAULT_QUERY_GC_TIME_MS,
        retry: DEFAULT_QUERY_RETRY_COUNT,
      },
      mutations: {
        retry: false,
      },
    },
  });
  return {
    queryClient,
  };
}

export { getContext };
