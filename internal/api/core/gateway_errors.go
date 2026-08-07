package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/gateway"
)

const GatewayInvalidRequestCode = "gateway_invalid_request"

var gatewayErrorCodes = [...]struct {
	target error
	code   string
}{
	{errGatewayInvalidRequest, GatewayInvalidRequestCode},
	{gateway.ErrDeviceUnauthenticated, "gateway_device_unauthenticated"},
	{gateway.ErrDeviceNotFound, "gateway_device_not_found"},
	{gateway.ErrDeviceRevoked, "gateway_device_revoked"},
	{gateway.ErrPairingInvalid, "gateway_pairing_invalid"},
	{gateway.ErrPairingExpired, "gateway_pairing_expired"},
	{gateway.ErrPairingSpent, "gateway_pairing_spent"},
	{gateway.ErrPairingLimit, "gateway_pairing_limit"},
	{gateway.ErrPairingForbidden, "gateway_pairing_forbidden"},
	{gateway.ErrStreamTicketInvalid, "gateway_stream_ticket_invalid"},
	{gateway.ErrAuthRateLimited, "gateway_auth_rate_limited"},
	{gateway.ErrConsentRequired, "gateway_consent_required"},
	{gateway.ErrExposureRefused, "gateway_exposure_refused"},
	{gateway.ErrProviderTrustStale, "gateway_provider_trust_stale"},
	{gateway.ErrProviderNotFound, "gateway_provider_not_found"},
	{gateway.ErrDigestConfirmationRequired, "gateway_digest_confirmation_required"},
	{gateway.ErrIngressForbidden, "gateway_ingress_forbidden"},
	{gateway.ErrIngressSubjectNotFound, "gateway_ingress_subject_not_found"},
	{gateway.ErrLocalOnlyOperation, "gateway_local_only_operation"},
	{gateway.ErrGenerationConflict, "gateway_generation_conflict"},
	{gateway.ErrTierProviderConflict, "gateway_tier_provider_conflict"},
}

// StatusForGatewayError maps gateway-domain failures to stable transport statuses.
func StatusForGatewayError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, errGatewayInvalidRequest):
		return http.StatusBadRequest
	case errors.Is(err, gateway.ErrDeviceUnauthenticated),
		errors.Is(err, gateway.ErrDeviceRevoked),
		errors.Is(err, gateway.ErrPairingInvalid),
		errors.Is(err, gateway.ErrStreamTicketInvalid):
		return http.StatusUnauthorized
	case errors.Is(err, gateway.ErrPairingForbidden),
		errors.Is(err, gateway.ErrIngressForbidden),
		errors.Is(err, gateway.ErrLocalOnlyOperation):
		return http.StatusForbidden
	case errors.Is(err, gateway.ErrDeviceNotFound),
		errors.Is(err, gateway.ErrProviderNotFound),
		errors.Is(err, gateway.ErrIngressSubjectNotFound):
		return http.StatusNotFound
	case errors.Is(err, gateway.ErrPairingExpired):
		return http.StatusGone
	case errors.Is(err, gateway.ErrPairingSpent),
		errors.Is(err, gateway.ErrExposureRefused),
		errors.Is(err, gateway.ErrProviderTrustStale),
		errors.Is(err, gateway.ErrGenerationConflict),
		errors.Is(err, gateway.ErrTierProviderConflict):
		return http.StatusConflict
	case errors.Is(err, gateway.ErrConsentRequired),
		errors.Is(err, gateway.ErrDigestConfirmationRequired):
		return http.StatusPreconditionRequired
	case errors.Is(err, gateway.ErrPairingLimit),
		errors.Is(err, gateway.ErrAuthRateLimited):
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// GatewayErrorCode returns the deterministic machine code for a known gateway error.
func GatewayErrorCode(err error) string {
	for _, candidate := range &gatewayErrorCodes {
		if errors.Is(err, candidate.target) {
			return candidate.code
		}
	}
	return ""
}
