import { queryOptions } from "@tanstack/react-query";

import { fetchComputer, fetchComputers } from "../adapters/computers-api";
import { computerRegistryKeys } from "./query-keys";

export function computersListOptions(enabled = true) {
  return queryOptions({
    queryKey: computerRegistryKeys.list(),
    queryFn: ({ signal }) => fetchComputers(signal),
    enabled,
    staleTime: 15_000,
    retry: false,
  });
}

export function computerDetailOptions(id: string, enabled = true) {
  return queryOptions({
    queryKey: computerRegistryKeys.detail(id),
    queryFn: ({ signal }) => fetchComputer(id, signal),
    enabled: enabled && id.trim() !== "",
    staleTime: 15_000,
    retry: false,
  });
}
