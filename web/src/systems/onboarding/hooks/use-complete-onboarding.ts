import { useMutation, useQueryClient } from "@tanstack/react-query";

import { completeOnboarding } from "../adapters/onboarding-api";
import { onboardingKeys } from "../lib/query-keys";
import type { OnboardingStatus } from "../types";

export function useCompleteOnboarding() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => completeOnboarding(),
    onSuccess: status => {
      queryClient.setQueryData<OnboardingStatus>(onboardingKeys.status(), status);
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: onboardingKeys.status() });
    },
  });
}
