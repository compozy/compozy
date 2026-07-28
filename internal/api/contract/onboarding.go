package contract

// OnboardingStatusPayload describes the global first-run onboarding completion state.
type OnboardingStatusPayload struct {
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// OnboardingStatusResponse wraps the onboarding completion state.
type OnboardingStatusResponse struct {
	Onboarding OnboardingStatusPayload `json:"onboarding"`
}
