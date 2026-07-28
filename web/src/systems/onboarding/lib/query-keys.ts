import type { DirectoryBrowseQuery } from "../types";

export const onboardingKeys = {
  all: ["onboarding"] as const,
  status: () => [...onboardingKeys.all, "status"] as const,
  browse: (query: DirectoryBrowseQuery) => [...onboardingKeys.all, "browse", query] as const,
};
