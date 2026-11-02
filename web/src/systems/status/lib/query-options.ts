import { queryOptions } from "@tanstack/react-query";

import { fetchRuntimeIdentity, fetchStatus } from "../adapters/daemon-api";
import { statusKeys } from "./query-keys";

export function statusOptions(enabled = true) {
  return queryOptions({
    queryKey: statusKeys.current(),
    queryFn: ({ signal }) => fetchStatus(signal),
    enabled,
    refetchInterval: 10_000,
    retry: 1,
    staleTime: 5_000,
  });
}

export function runtimeIdentityOptions(enabled = true) {
  return queryOptions({
    queryKey: statusKeys.identity(),
    queryFn: ({ signal }) => fetchRuntimeIdentity(signal),
    enabled,
    refetchInterval: 10_000,
    retry: 1,
    staleTime: 5_000,
  });
}
