package providerusage

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var _ error = (*UsageError)(nil) //nolint:errcheck // Compile-time interface assertion, not an error result.

func (e *UsageError) Error() string { return e.Message }

type errorDescription struct{ message, action string }

var errorDescriptions = map[ErrorCode]errorDescription{
	ErrorCodeAuthMissing: {message: "Sign in with %s to read usage", action: string(ActionSignIn)},
	ErrorCodeAuthExpired: {message: "Run %s to refresh your login", action: string(ActionSignIn)},
	ErrorCodeAuthScope: {
		message: "This %s login can't read usage. Sign in with %s to enable it.",
		action:  string(ActionSignIn),
	},
	ErrorCodeKeychainConsentRequired: {message: "Needs Keychain access", action: string(ActionAllowKeychain)},
	ErrorCodeKeychainDenied:          {message: "Keychain access denied", action: string(ActionAllowKeychain)},
	ErrorCodeKeychainReadTimeout: {
		message: "Background Keychain read timed out; an authorization prompt may have been required",
		action:  string(ActionRefresh),
	},
	ErrorCodeKeychainTokenExpired: {message: "Refresh to reload your Claude login", action: string(ActionRefresh)},
	ErrorCodeProviderRateLimited:  {message: "Rate limited by %s", action: string(ActionRetry)},
	ErrorCodeProviderUnavailable:  {message: "%s usage service is unavailable", action: string(ActionRetry)},
	ErrorCodeNetworkError:         {message: "Couldn't reach %s", action: string(ActionRetry)},
	ErrorCodeParseError:           {message: "Unexpected response from %s", action: string(ActionRetry)},
	ErrorCodeNoCollector:          {message: "Usage not available for this provider", action: ""},
	ErrorCodeNoPlan:               {message: "No coding plan on this key", action: string(ActionConfigure)},
	ErrorCodeAuthStoreUnsupported: {
		message: "Codex keeps its login in the system keyring; usage needs cli_auth_credentials_store = \"file\"",
		action:  string(ActionConfigure),
	},
	ErrorCodeDisabled: {message: "Turned off in settings", action: string(ActionConfigure)},
	ErrorCodeProviderUsageDisabled: {
		message: "provider usage is turned off",
		action:  "compozy config set provider_usage.enabled true",
	},
	ErrorCodeProviderNotFound:       {message: "provider %q not found", action: "use a canonical provider ID"},
	ErrorCodeProfileNotFound:        {message: "profile %q not found", action: "pick an existing profile"},
	ErrorCodeProfileSessionConflict: {message: "profile does not match the session's profile", action: "omit profile"},
	ErrorCodeAppTokenRouteForbidden: {
		message: "route not available to this app token",
		action:  "refresh from the shell or CLI",
	},
	ErrorCodeInvalidRequest: {message: "invalid provider usage request", action: "fix the argument"},
}

// NewUsageError never includes upstream error text or response bodies.
func NewUsageError(code ErrorCode, provider string) *UsageError {
	description := errorDescriptions[code]
	message := description.message
	if strings.Contains(message, "%") {
		if strings.Count(message, "%s") == 2 {
			message = fmt.Sprintf(message, provider, provider)
		} else {
			message = fmt.Sprintf(message, provider)
		}
	}
	if strings.EqualFold(provider, claudeProviderID) {
		switch code {
		case ErrorCodeAuthMissing:
			message = "Sign in with claude /login to read usage"
		case ErrorCodeAuthExpired:
			message = "Run claude /login to sign in again"
		case ErrorCodeAuthScope:
			message = "This Claude login can't read usage. Run claude /login to sign in again."
		}
	}
	return &UsageError{Code: code, Message: message, Action: description.action}
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
	case ErrorCodeKeychainConsentRequired, ErrorCodeKeychainDenied, ErrorCodeKeychainTokenExpired, ErrorCodeKeychainReadTimeout:
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
	if usage, ok := errors.AsType[*UsageError](err); ok {
		return usage
	}
	return NewUsageError(ErrorCodeProviderUnavailable, provider)
}
