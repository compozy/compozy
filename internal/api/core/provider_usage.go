package core

import (
	"errors"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/providerusage"
	"github.com/gin-gonic/gin"
)

// ListProviderUsage reads only the profile-scoped cache.
func (h *BaseHandlers) ListProviderUsage(c *gin.Context) {
	profile, ok := h.providerUsageProfile(c)
	if !ok || !h.requireProviderUsage(c) {
		return
	}
	envelope, err := h.ProviderUsage.Envelope(c.Request.Context(), profile)
	if err == nil && c.Param("provider_id") != "" {
		envelope, err = FilterProviderUsage(envelope, c.Param("provider_id"))
	}
	if err != nil {
		h.respondProviderUsageError(c, err)
		return
	}
	c.JSON(http.StatusOK, ProviderUsageEnvelopePayload(envelope))
}

// FilterProviderUsage preserves the envelope shape for a single provider read.
func FilterProviderUsage(envelope providerusage.Envelope, providerID string) (providerusage.Envelope, error) {
	for _, target := range envelope.Targets {
		if target.ProviderID == providerID {
			envelope.Targets = []providerusage.Snapshot{target}
			envelope.Tray = nil
			return envelope, nil
		}
	}
	return providerusage.Envelope{}, providerusage.NewUsageError(providerusage.ErrorCodeProviderNotFound, providerID)
}

func (h *BaseHandlers) RefreshProviderUsage(c *gin.Context) {
	var input contract.ProviderUsageRefreshRequest
	if err := c.ShouldBindJSON(&input); err != nil && !errors.Is(err, io.EOF) {
		h.respondProviderUsageError(c, invalidProviderUsageRequest("invalid provider usage refresh request"))
		return
	}
	req, err := ProviderUsageRefreshInput(input)
	if err != nil {
		h.respondProviderUsageError(c, err)
		return
	}
	// The body is authoritative; the shared profile resolver still enforces session binding.
	query := c.Request.URL.Query()
	query.Del("profile")
	if input.Profile != "" {
		query.Set("profile", input.Profile)
	}
	c.Request.URL.RawQuery = query.Encode()
	profile, ok := h.providerUsageProfile(c)
	if !ok || !h.requireProviderUsage(c) {
		return
	}
	req.ProfileID = profile
	req.Intent = providerUsageIntent(c, req.Intent)
	envelope, err := h.ProviderUsage.Refresh(c.Request.Context(), req)
	if err != nil {
		h.respondProviderUsageError(c, err)
		return
	}
	c.JSON(http.StatusOK, ProviderUsageEnvelopePayload(envelope))
}

// ProviderUsageRefreshInput validates transport bounds before duration conversion.
func ProviderUsageRefreshInput(input contract.ProviderUsageRefreshRequest) (providerusage.RefreshRequest, error) {
	wait := int64(15000)
	if input.WaitMS != nil {
		wait = *input.WaitMS
	}
	if wait < 0 || wait > 30000 {
		return providerusage.RefreshRequest{}, invalidProviderUsageRequest("wait_ms must be between 0 and 30000")
	}
	if input.MaxAgeSeconds < 0 {
		return providerusage.RefreshRequest{}, invalidProviderUsageRequest("max_age_seconds must be ≥ 0")
	}
	if input.MaxAgeSeconds > math.MaxInt64/int64(time.Second) {
		return providerusage.RefreshRequest{}, invalidProviderUsageRequest("max_age_seconds is too large")
	}
	intent := providerusage.Intent(input.Intent)
	if intent == "" {
		intent = providerusage.IntentBackground
	}
	if intent != providerusage.IntentBackground && intent != providerusage.IntentExplicit {
		return providerusage.RefreshRequest{}, invalidProviderUsageRequest("intent must be background or explicit")
	}
	return providerusage.RefreshRequest{ProfileID: input.Profile, ProviderIDs: input.ProviderIDs, MaxAge: time.Duration(input.MaxAgeSeconds) * time.Second, Wait: time.Duration(wait) * time.Millisecond, Intent: intent}, nil
}

func providerUsageIntent(c *gin.Context, requested providerusage.Intent) providerusage.Intent {
	if hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		return providerusage.IntentBackground
	}
	if _, ok := gateway.DeviceFromContext(c.Request.Context()); ok {
		return providerusage.IntentBackground
	}
	if _, ok := AppTokenFromContext(c.Request.Context()); ok {
		return providerusage.IntentBackground
	}
	return requested
}

func (h *BaseHandlers) providerUsageProfile(c *gin.Context) (string, bool) {
	all, err := parseBoolQuery(c, "all_profiles")
	if err != nil || all {
		h.respondProviderUsageError(c, invalidProviderUsageRequest("provider usage is per profile; all_profiles is not supported"))
		return "", false
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return "", false
	}
	return scope.ProfileID, true
}

func (h *BaseHandlers) requireProviderUsage(c *gin.Context) bool {
	if h.ProviderUsage != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, contract.ErrorPayload{Error: "provider usage is unavailable", Code: "provider_usage_unavailable"})
	return false
}

func invalidProviderUsageRequest(message string) *providerusage.UsageError {
	return &providerusage.UsageError{Code: providerusage.ErrorCodeInvalidRequest, Message: message}
}

func (h *BaseHandlers) respondProviderUsageError(c *gin.Context, err error) {
	var usage *providerusage.UsageError
	if !errors.As(err, &usage) {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	status := http.StatusInternalServerError
	switch usage.Code {
	case providerusage.ErrorCodeInvalidRequest:
		status = http.StatusBadRequest
	case providerusage.ErrorCodeProviderNotFound:
		status = http.StatusNotFound
	case providerusage.ErrorCodeProviderUsageDisabled:
		status = http.StatusConflict
	}
	c.JSON(status, contract.ErrorPayload{Error: usage.Message, Code: string(usage.Code)})
}
