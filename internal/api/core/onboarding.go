package core

import (
	"errors"
	"net/http"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

// GetOnboardingStatus reports whether first-run onboarding has been completed.
func (h *BaseHandlers) GetOnboardingStatus(c *gin.Context) {
	onboarding, err := h.onboardingStore()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	status, err := onboarding.GetOnboardingStatus(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, onboardingStatusResponse(status))
}

// CompleteOnboarding marks first-run onboarding as completed and returns the new status.
func (h *BaseHandlers) CompleteOnboarding(c *gin.Context) {
	onboarding, err := h.onboardingStore()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	completedAt := h.Now().UTC().Format(time.RFC3339)
	status, err := onboarding.CompleteOnboarding(c.Request.Context(), completedAt)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, onboardingStatusResponse(status))
}

// ResetOnboarding clears the onboarding completion flag so the wizard runs again.
func (h *BaseHandlers) ResetOnboarding(c *gin.Context) {
	onboarding, err := h.onboardingStore()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	status, err := onboarding.ResetOnboarding(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, onboardingStatusResponse(status))
}

func (h *BaseHandlers) onboardingStore() (OnboardingStore, error) {
	if h.Onboarding == nil {
		return nil, errors.New("api: onboarding store is not configured")
	}
	return h.Onboarding, nil
}

func onboardingStatusResponse(status store.OnboardingStatus) contract.OnboardingStatusResponse {
	return contract.OnboardingStatusResponse{
		Onboarding: contract.OnboardingStatusPayload{
			Completed:   status.Completed,
			CompletedAt: status.CompletedAt,
		},
	}
}
