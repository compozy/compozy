package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/gateway"
)

const GatewayInvalidRequestCode = "gateway_invalid_request"

var gatewayErrorMappings = [...]struct {
	target error
	status int
	code   string
}{
	{errGatewayInvalidRequest, http.StatusBadRequest, GatewayInvalidRequestCode},
	{gateway.ErrDeviceUnauthenticated, http.StatusUnauthorized, "gateway_device_unauthenticated"},
	{gateway.ErrDeviceNotFound, http.StatusNotFound, "gateway_device_not_found"},
	{gateway.ErrDeviceRevoked, http.StatusUnauthorized, "gateway_device_revoked"},
	{gateway.ErrPairingInvalid, http.StatusUnauthorized, "gateway_pairing_invalid"},
	{gateway.ErrPairingExpired, http.StatusGone, "gateway_pairing_expired"},
	{gateway.ErrPairingSpent, http.StatusConflict, "gateway_pairing_spent"},
	{gateway.ErrPairingLimit, http.StatusTooManyRequests, "gateway_pairing_limit"},
	{gateway.ErrPairingForbidden, http.StatusForbidden, "gateway_pairing_forbidden"},
	{gateway.ErrStreamTicketInvalid, http.StatusUnauthorized, "gateway_stream_ticket_invalid"},
	{gateway.ErrStreamTicketLimit, http.StatusTooManyRequests, "gateway_stream_ticket_limit"},
	{gateway.ErrAuthRateLimited, http.StatusTooManyRequests, "gateway_auth_rate_limited"},
	{gateway.ErrConsentRequired, http.StatusPreconditionRequired, "gateway_consent_required"},
	{gateway.ErrExposureRefused, http.StatusConflict, "gateway_exposure_refused"},
	{gateway.ErrProviderTrustStale, http.StatusConflict, "gateway_provider_trust_stale"},
	{gateway.ErrProviderNotFound, http.StatusNotFound, "gateway_provider_not_found"},
	{gateway.ErrDigestConfirmationRequired, http.StatusPreconditionRequired, "gateway_digest_confirmation_required"},
	{gateway.ErrEndpointUnverified, http.StatusConflict, "gateway_endpoint_unverified"},
	{gateway.ErrProviderDegraded, http.StatusServiceUnavailable, "gateway_provider_degraded"},
	{gateway.ErrIngressForbidden, http.StatusForbidden, "gateway_ingress_forbidden"},
	{gateway.ErrIngressSubjectNotFound, http.StatusNotFound, "gateway_ingress_subject_not_found"},
	{gateway.ErrIngressTargetUnavailable, http.StatusConflict, "gateway_ingress_target_unavailable"},
	{gateway.ErrIngressRateLimited, http.StatusTooManyRequests, "gateway_ingress_rate_limited"},
	{gateway.ErrLocalOnlyOperation, http.StatusForbidden, "gateway_local_only_operation"},
	{gateway.ErrGenerationConflict, http.StatusConflict, "gateway_generation_conflict"},
	{gateway.ErrTierProviderConflict, http.StatusConflict, "gateway_tier_provider_conflict"},
}

// StatusForGatewayError maps gateway-domain failures to stable transport statuses.
func StatusForGatewayError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	for _, mapping := range &gatewayErrorMappings {
		if errors.Is(err, mapping.target) {
			return mapping.status
		}
	}
	return http.StatusInternalServerError
}

// GatewayErrorCode returns the deterministic machine code for a known gateway error.
func GatewayErrorCode(err error) string {
	for _, candidate := range &gatewayErrorMappings {
		if errors.Is(err, candidate.target) {
			return candidate.code
		}
	}
	return ""
}
