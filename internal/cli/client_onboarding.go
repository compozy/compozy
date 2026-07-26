package cli

import (
	"context"
	"net/http"

	"github.com/compozy/agh/internal/api/contract"
)

func (c *unixSocketClient) GetOnboardingStatus(ctx context.Context) (contract.OnboardingStatusResponse, error) {
	var response contract.OnboardingStatusResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/onboarding", nil, nil, &response); err != nil {
		return contract.OnboardingStatusResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) CompleteOnboarding(ctx context.Context) (contract.OnboardingStatusResponse, error) {
	var response contract.OnboardingStatusResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/onboarding/complete", nil, nil, &response); err != nil {
		return contract.OnboardingStatusResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ResetOnboarding(ctx context.Context) (contract.OnboardingStatusResponse, error) {
	var response contract.OnboardingStatusResponse
	if err := c.doJSON(ctx, http.MethodDelete, "/api/onboarding", nil, nil, &response); err != nil {
		return contract.OnboardingStatusResponse{}, err
	}
	return response, nil
}
