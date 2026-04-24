import { QueryClient } from "@tanstack/react-query";

import { isStaleWorkspaceError } from "./api-client";

export function createDaemonQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        refetchOnWindowFocus: false,
        retry: (failureCount, error) => {
          if (isStaleWorkspaceError(error)) {
            return false;
          }
          return failureCount < 2;
        },
        staleTime: 10_000,
      },
      mutations: {
        retry: false,
      },
    },
  });
}
