import { queryOptions } from "@tanstack/react-query";

import { LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import { browseDirectory, fetchOnboardingStatus } from "../adapters/onboarding-api";
import type { DirectoryBrowseQuery } from "../types";
import { onboardingKeys } from "./query-keys";

export function onboardingStatusOptions() {
  return queryOptions({
    queryKey: onboardingKeys.status(),
    queryFn: ({ signal }) => fetchOnboardingStatus(signal),
    staleTime: 30_000,
  });
}

export function directoryBrowseOptions(
  query: DirectoryBrowseQuery,
  enabled: boolean,
  computerId: ComputerId = LOCAL_COMPUTER_ID
) {
  const keys = onboardingKeys.forComputer(computerId);
  return queryOptions({
    queryKey: keys.browse(query),
    queryFn: ({ signal }) => browseDirectory(query, signal, computerId),
    enabled,
    staleTime: 10_000,
  });
}
