import { useQuery } from "@tanstack/react-query";

import { onboardingStatusOptions } from "../lib/query-options";

export function useOnboardingStatus() {
  return useQuery(onboardingStatusOptions());
}
