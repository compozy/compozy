package auth

import (
	"errors"
	"net/http"
)

type Failure struct {
	Status     int
	Code       string
	Message    string
	Reason     string
	RetryAfter int
}

func (e *Failure) Error() string { return e.Code }

func failure(status int, code, message string) *Failure {
	return &Failure{Status: status, Code: code, Message: message}
}

func reauth(reason string) *Failure {
	message := "Your session ended. Sign in again."
	switch reason {
	case errDeviceRevoked.Code:
		message = "This computer was removed from your account. Sign in again."
	case "account_deleted":
		message = "Your account was deleted."
	case errAccountSuspended.Code:
		message = errAccountSuspended.Message
	}
	return &Failure{Status: http.StatusUnauthorized, Code: "reauth_required",
		Message: message, Reason: reason}
}

var (
	ErrInvalidRequest = failure(http.StatusBadRequest, "invalid_request", "Check the sign-in details and try again.")
	ErrTurnstile      = failure(
		http.StatusBadRequest,
		"turnstile_required",
		"We couldn't verify this browser. Try again.",
	)
	errOAuthState = failure(http.StatusBadRequest, "sign_in_state_invalid", "Start a new sign-in from CompozyOS.")
	errProvider   = failure(http.StatusBadGateway, "sign_in_provider_failed", "Sign-in failed. Try again.")
	errFreshAuth  = failure(http.StatusForbidden, "fresh_sign_in_required",
		"Sign in again with Google in a private browser window to recover admin access.")
	errEmailUnverified  = failure(http.StatusForbidden, "sign_in_email_unverified", "Email verification required")
	errIdentityConflict = failure(http.StatusConflict, "sign_in_identity_conflict",
		"This email is linked to another account. Contact support@compozy.com.")
	errAccountSuspended = failure(http.StatusForbidden, "account_suspended",
		"This account is suspended. Contact support@compozy.com.")
	errAccountDeleting = failure(http.StatusConflict, "account_deleting",
		"Your account is being deleted. Try again in a few minutes.")
	errAccountTokenLocalOnly = failure(http.StatusForbidden, "account_token_local_only",
		"Account tokens unlock local use only. Sign in with Google or GitHub for this.")
	errDeviceRevoked = failure(http.StatusForbidden, "device_revoked", "This device was removed from your account.")
	errDenied        = failure(
		http.StatusBadRequest,
		"access_denied",
		"Sign-in was cancelled.", //nolint:misspell // Spec copy.
	)
	errExpired    = failure(http.StatusBadRequest, "expired_token", "Sign-in timed out. Try again.")
	errPending    = failure(http.StatusBadRequest, "authorization_pending", "Continue in your browser.")
	errSlowDown   = failure(http.StatusBadRequest, "slow_down", "Wait before checking for sign-in again.")
	errCodeUsed   = failure(http.StatusConflict, "device_code_used", "This code was already used.")
	errWrongCode  = failure(http.StatusBadRequest, "user_code_invalid", "Check the code shown on your computer.")
	errEnrollment = failure(
		http.StatusUnauthorized,
		"enrollment_token_invalid",
		"Start a new sign-in from CompozyOS.",
	)
	errKeyRevoked    = failure(http.StatusConflict, "device_key_revoked", "This device key was revoked.")
	errInvalidProof  = failure(http.StatusUnauthorized, "dpop_invalid", "Device key verification failed.")
	errReplayedProof = failure(http.StatusUnauthorized, "dpop_replayed", "Device key verification failed.")
	errWrongComputer = failure(http.StatusForbidden, "computer_not_owned", "This computer belongs to another account.")
	errPlan          = failure(http.StatusPaymentRequired, "plan_required", "This computer's hosted route needs Pro.")
	errKeyRebind     = failure(
		http.StatusConflict,
		"browser_key_rebind_required",
		"Sign in again to replace this browser key.",
	)
	errSessionInvalid = reauth("session_invalid")
	ErrWebSession     = errors.New("cloud web session is missing or expired")
)
