import { useQuery, useQueryClient } from "@tanstack/react-query";

import { networkMessageTailOptions, type NetworkMessageTailQuery } from "../lib/query-options";

export function useNetworkMessageTail(queryInput: NetworkMessageTailQuery): {
  isFetching: boolean;
  error: Error | null;
} {
  const queryClient = useQueryClient();
  const query = useQuery(networkMessageTailOptions(queryClient, queryInput));
  return { isFetching: query.isFetching, error: query.error ?? null };
}
