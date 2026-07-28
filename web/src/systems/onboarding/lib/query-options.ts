import { queryOptions } from "@tanstack/react-query";

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

export function directoryBrowseOptions(query: DirectoryBrowseQuery, enabled: boolean) {
  return queryOptions({
    queryKey: onboardingKeys.browse(query),
    queryFn: ({ signal }) => browseDirectory(query, signal),
    enabled,
    staleTime: 10_000,
  });
}
