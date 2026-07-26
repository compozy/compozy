package store

import "context"

// OnboardingStatus describes the global first-run onboarding completion state.
type OnboardingStatus struct {
	Completed   bool
	CompletedAt string
}

// OnboardingStore manages the domain lifecycle for first-run onboarding.
type OnboardingStore interface {
	GetOnboardingStatus(ctx context.Context) (OnboardingStatus, error)
	CompleteOnboarding(ctx context.Context, completedAt string) (OnboardingStatus, error)
	ResetOnboarding(ctx context.Context) (OnboardingStatus, error)
}
