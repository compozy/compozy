import type { OnboardingStatus } from "../types";

export const onboardingCompletedFixture: OnboardingStatus = {
  completed: true,
  completed_at: "2026-04-17T18:00:00Z",
};

export const onboardingIncompleteFixture: OnboardingStatus = {
  completed: false,
};
