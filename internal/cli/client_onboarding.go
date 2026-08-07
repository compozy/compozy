package cli

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) GetOnboardingStatus(ctx context.Context) (contract.OnboardingStatusResponse, error) {
	var response contract.OnboardingStatusResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/onboarding", nil, nil, &response); err != nil {
		return contract.OnboardingStatusResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) CompleteOnboarding(ctx context.Context) (contract.OnboardingStatusResponse, error) {
	var response contract.OnboardingStatusResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/onboarding/complete", nil, nil, &response); err != nil {
		return contract.OnboardingStatusResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) ResetOnboarding(ctx context.Context) (contract.OnboardingStatusResponse, error) {
	var response contract.OnboardingStatusResponse
	if err := c.doJSON(ctx, http.MethodDelete, "/api/onboarding", nil, nil, &response); err != nil {
		return contract.OnboardingStatusResponse{}, err
	}
	return response, nil
}
