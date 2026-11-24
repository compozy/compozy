package remotehost

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/remoteproto"
)

const invalidRemoteRequestCode = "invalid_remote_request"

var (
	// ErrDeviceRevoked reports an epoch fence invalidated by revocation.
	ErrDeviceRevoked = errors.New("gateway device revoked")
	// ErrStreamTicketInvalid masks unknown, malformed, expired, spent, and revoked-device tickets.
	ErrStreamTicketInvalid = errors.New("gateway stream ticket invalid")
	// ErrStreamTicketLimit reports that the bounded live-ticket set is full.
	ErrStreamTicketLimit = errors.New("gateway stream ticket limit reached")
	// ErrAuthRateLimited reports too many failed authentication attempts from one source.
	ErrAuthRateLimited = errors.New("gateway authentication rate limited")
)

type Failure struct {
	Status  int
	Code    string
	Message string
	Details map[string]string
}

func (e *Failure) Error() string { return e.Code }

var (
	ErrNotLinked   = &Failure{http.StatusConflict, "remote_not_linked", "Sign in to turn on remote access.", nil}
	ErrEntitlement = &Failure{
		http.StatusServiceUnavailable,
		"remote_entitlement_unavailable",
		"Remote access paused: entitlement unavailable.",
		nil,
	}
	ErrPlanRequired = &Failure{
		http.StatusPaymentRequired,
		"plan_required",
		"The plan does not allow this route.",
		nil,
	}
	ErrSnapshotPending = &Failure{
		http.StatusServiceUnavailable,
		"security_snapshot_pending",
		"The security snapshot is pending.",
		nil,
	}
	ErrRouteMismatch = &Failure{
		http.StatusForbidden,
		"route_mismatch",
		"The grant does not match the arrival route.",
		nil,
	}
	ErrGrantReplay    = &Failure{http.StatusForbidden, "grant_replayed", "The grant has already been used.", nil}
	ErrSessionExpired = &Failure{
		http.StatusUnauthorized,
		"remote_session_expired",
		"The remote session expired.",
		nil,
	}
	ErrSessionInvalid = &Failure{
		http.StatusUnauthorized,
		"remote_session_invalid",
		"A valid remote session is required.",
		nil,
	}
	ErrApprovalExpired = &Failure{
		http.StatusForbidden,
		"device_approval_expired",
		"Approval request expired. Try again.",
		nil,
	}
	ErrApprovalDenied = &Failure{
		http.StatusForbidden,
		"device_not_approved",
		"This computer didn't approve this device.",
		nil,
	}
	ErrApprovalNotPending = &Failure{
		http.StatusConflict,
		"approval_not_pending",
		"The approval request is no longer pending.",
		nil,
	}
	ErrRouteNotRemote = &Failure{
		http.StatusNotFound,
		"route_not_remote",
		"This route is not available remotely.",
		nil,
	}
	ErrClaimTokenNotRemote = &Failure{
		http.StatusBadRequest,
		"claim_token_not_remote",
		"Task leases stay on the computer that owns them.",
		nil,
	}
	ErrRemoteCallerOverride = &Failure{
		http.StatusBadRequest,
		"remote_caller_override_forbidden",
		"Remote tool calls use the authenticated device identity.",
		nil,
	}
	ErrDeviceNotFound = &Failure{http.StatusNotFound, "device_not_found", "The device was not found.", nil}
)

func DescribeError(err error) Failure {
	if failure, ok := errors.AsType[*Failure](err); ok {
		return *failure
	}
	switch {
	case errors.Is(err, ErrDeviceRevoked):
		return Failure{http.StatusForbidden, "device_revoked", "This device was removed from your account.", nil}
	case errors.Is(err, remoteproto.ErrDPoPInvalid), errors.Is(err, remoteproto.ErrDPoPReplayed):
		return Failure{http.StatusUnauthorized, err.Error(), "The proof of possession was rejected.", nil}
	case errors.Is(err, remoteproto.ErrGrantInvalid), errors.Is(err, remoteproto.ErrGrantExpired),
		errors.Is(err, remoteproto.ErrGrantBindingMissing), errors.Is(err, remoteproto.ErrGrantSignatureInvalid):
		return Failure{http.StatusForbidden, err.Error(), "The remote grant was rejected.", nil}
	default:
		return Failure{http.StatusInternalServerError, "remote_operation_failed", "The remote operation failed.", nil}
	}
}
