import { queryOptions } from "@tanstack/react-query";

import { fetchStatus } from "../adapters/daemon-api";
import { statusKeys } from "./query-keys";

export function statusOptions() {
  return queryOptions({
    queryKey: statusKeys.current(),
    queryFn: ({ signal }) => fetchStatus(signal),
    refetchInterval: 10_000,
    retry: 1,
    staleTime: 5_000,
  });
}
