import { QueryClient } from "@tanstack/react-query";

export function createDaemonQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        refetchOnWindowFocus: false,
        retry: (failureCount, error) => {
          if (isStaleWorkspaceErrorFromUnknown(error)) {
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

function isStaleWorkspaceErrorFromUnknown(error: unknown): boolean {
  if (!error || typeof error !== "object") {
    return false;
  }
  const code = Reflect.get(error, "code");
  return code === "workspace_context_stale";
}
