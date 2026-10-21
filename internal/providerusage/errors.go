package providerusage

import (
	"errors"
	"fmt"
	"time"
)

var _ error = (*UsageError)(nil)

func (e *UsageError) Error() string { return e.Message }

// NewUsageError never includes upstream error text or response bodies.
func NewUsageError(code ErrorCode, provider string) *UsageError {
	e := &UsageError{Code: code}
	switch code {
	case ErrorCodeAuthMissing:
		e.Message = "Sign in with " + provider + " to read usage"
		e.Action = string(ActionSignIn)
	case ErrorCodeAuthExpired:
		e.Message = "Run " + provider + " to refresh your login"
		e.Action = string(ActionSignIn)
	case ErrorCodeAuthScope:
		e.Message = "This " + provider + " login can't read usage. Sign in with " + provider + " to enable it."
		e.Action = string(ActionSignIn)
	case ErrorCodeKeychainConsentRequired:
		e.Message = "Needs Keychain access"
		e.Action = string(ActionAllowKeychain)
	case ErrorCodeKeychainDenied:
		e.Message = "Keychain access denied"
		e.Action = string(ActionAllowKeychain)
	case ErrorCodeKeychainTokenExpired:
		e.Message = "Refresh to reload your Claude login"
		e.Action = string(ActionRefresh)
	case ErrorCodeProviderRateLimited:
		e.Message = "Rate limited by " + provider
		e.Action = string(ActionRetry)
	case ErrorCodeProviderUnavailable:
		e.Message = provider + " usage service is unavailable"
		e.Action = string(ActionRetry)
	case ErrorCodeNetworkError:
		e.Message = "Couldn't reach " + provider
		e.Action = string(ActionRetry)
	case ErrorCodeParseError:
		e.Message = "Unexpected response from " + provider
		e.Action = string(ActionRetry)
	case ErrorCodeNoCollector:
		e.Message = "Usage not available for this provider"
	case ErrorCodeNoPlan:
		e.Message = "No coding plan on this key"
		e.Action = string(ActionConfigure)
	case ErrorCodeAuthStoreUnsupported:
		e.Message = "Codex keeps its login in the system keyring; usage needs cli_auth_credentials_store = \"file\""
		e.Action = string(ActionConfigure)
	case ErrorCodeDisabled:
		e.Message = "Turned off in settings"
		e.Action = string(ActionConfigure)
	case ErrorCodeProviderUsageDisabled:
		e.Message = "provider usage is turned off"
		e.Action = "compozy config set provider_usage.enabled true"
	case ErrorCodeProviderNotFound:
		e.Message = fmt.Sprintf("provider %q not found", provider)
		e.Action = "use a canonical provider ID"
	case ErrorCodeProfileNotFound:
		e.Message = fmt.Sprintf("profile %q not found", provider)
		e.Action = "pick an existing profile"
	case ErrorCodeProfileSessionConflict:
		e.Message = "profile does not match the session's profile"
		e.Action = "omit profile"
	case ErrorCodeAppTokenRouteForbidden:
		e.Message = "route not available to this app token"
		e.Action = "refresh from the shell or CLI"
	case ErrorCodeInvalidRequest:
		e.Message = "invalid provider usage request"
		e.Action = "fix the argument"
	}
	return e
}

func RateLimited(provider string, until time.Time) *UsageError {
	e := NewUsageError(ErrorCodeProviderRateLimited, provider)
	e.RetryAfter = &until
	e.Message += " · retry after " + until.Format("15:04")
	return e
}

func (e *UsageError) Status() Status {
	switch e.Code {
	case ErrorCodeAuthMissing, ErrorCodeAuthExpired, ErrorCodeAuthScope:
		return StatusSignedOut
	case ErrorCodeKeychainConsentRequired, ErrorCodeKeychainDenied, ErrorCodeKeychainTokenExpired:
		return StatusNeedsConsent
	case ErrorCodeProviderRateLimited:
		return StatusRateLimited
	case ErrorCodeNoCollector, ErrorCodeNoPlan, ErrorCodeAuthStoreUnsupported:
		return StatusUnsupported
	case ErrorCodeDisabled:
		return StatusDisabled
	default:
		return StatusError
	}
}

func AsUsageError(err error, provider string) *UsageError {
	var usage *UsageError
	if errors.As(err, &usage) {
		return usage
	}
	return NewUsageError(ErrorCodeProviderUnavailable, provider)
}
