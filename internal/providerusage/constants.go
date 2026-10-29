package providerusage

type Status string

const (
	StatusOK           Status = "ok"
	StatusStale        Status = "stale"
	StatusRefreshing   Status = "refreshing"
	StatusSignedOut    Status = "signed_out"
	StatusNeedsConsent Status = "needs_consent"
	StatusRateLimited  Status = "rate_limited"
	StatusError        Status = "error"
	StatusUnsupported  Status = "unsupported"
	StatusDisabled     Status = "disabled"
)

type Action string

const (
	ActionSignIn        Action = "sign_in"
	ActionAllowKeychain Action = "allow_keychain"
	ActionRefresh       Action = "refresh"
	ActionRetry         Action = "retry"
	ActionConfigure     Action = "configure"
	ActionNone          Action = ""
)

type SourceKind string

const (
	SourceKindCLIRPC      SourceKind = "cli_rpc"
	SourceKindOAuthHTTP   SourceKind = "oauth_http"
	SourceKindAPIKeyHTTP  SourceKind = "api_key_http" //nolint:gosec // Public source-kind identifier, not a credential.
	SourceKindACPObserved SourceKind = "acp_observed"
)

type WindowKind string

const (
	WindowKindSession WindowKind = "session"
	WindowKindDaily   WindowKind = "daily"
	WindowKindWeekly  WindowKind = "weekly"
	WindowKindMonthly WindowKind = "monthly"
	WindowKindModel   WindowKind = "model"
	WindowKindOther   WindowKind = "other"
)

type Severity string

const (
	SeverityNeutral   Severity = "neutral"
	SeverityWarning   Severity = "warning"
	SeverityExhausted Severity = "exhausted"
)

type Confidence string

const (
	ConfidenceExact       Confidence = "exact"
	ConfidencePercentOnly Confidence = "percent_only"
	ConfidenceEstimated   Confidence = "estimated"
)

type ErrorCode string

const (
	ErrorCodeProfileNotFound         ErrorCode = "profile_not_found"
	ErrorCodeProfileSessionConflict  ErrorCode = "profile_session_conflict"
	ErrorCodeInvalidRequest          ErrorCode = "invalid_request"
	ErrorCodeAppTokenRouteForbidden  ErrorCode = "app_token_route_forbidden" //nolint:gosec // Public error code.
	ErrorCodeProviderNotFound        ErrorCode = "provider_not_found"
	ErrorCodeProviderUsageDisabled   ErrorCode = "provider_usage_disabled"
	ErrorCodeAuthMissing             ErrorCode = "auth_missing"
	ErrorCodeAuthExpired             ErrorCode = "auth_expired"
	ErrorCodeAuthScope               ErrorCode = "auth_scope"
	ErrorCodeKeychainConsentRequired ErrorCode = "keychain_consent_required"
	ErrorCodeKeychainDenied          ErrorCode = "keychain_denied"
	ErrorCodeKeychainReadTimeout     ErrorCode = "keychain_read_timeout"
	ErrorCodeKeychainTokenExpired    ErrorCode = "keychain_token_expired"
	ErrorCodeProviderRateLimited     ErrorCode = "provider_rate_limited"
	ErrorCodeProviderUnavailable     ErrorCode = "provider_unavailable"
	ErrorCodeNetworkError            ErrorCode = "network_error"
	ErrorCodeParseError              ErrorCode = "parse_error"
	ErrorCodeNoCollector             ErrorCode = "no_collector"
	ErrorCodeNoPlan                  ErrorCode = "no_plan"
	ErrorCodeAuthStoreUnsupported    ErrorCode = "auth_store_unsupported"
	ErrorCodeDisabled                ErrorCode = "disabled"
)

const (
	headerAuthorization = "Authorization"
	headerAccept        = "Accept"
	collectorJSONType   = "application/json"
	sessionWindowLabel  = "Session"
	weeklyWindowLabel   = "Weekly"
	jsonNull            = "null"
	creditsUnit         = "credits"
)
