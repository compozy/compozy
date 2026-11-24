import { queryOptions } from "@tanstack/react-query";

import { fetchAccountStatus } from "../adapters/account-api";
import { accountKeys } from "./query-keys";

export function accountStatusOptions(enabled = true) {
  return queryOptions({
    queryKey: accountKeys.status(),
    queryFn: ({ signal }) => fetchAccountStatus(signal),
    enabled,
    staleTime: 30_000,
    retry: false,
  });
}
