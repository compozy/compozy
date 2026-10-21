package providerusage

import "time"

type Envelope struct {
	KeychainSupported bool
	ProfileID         string
	Revision          uint64 // monotonic per profile within one daemon process
	GeneratedAt       time.Time
	Enabled           bool
	Tray              *TraySignal // nil when no known window
	Targets           []Snapshot  // ordered by provider catalog order
}

type TraySignal struct {
	ProviderID  string
	WindowID    string
	Label       string // "Codex · Weekly"
	UsedPercent float64
	ResetsAt    *time.Time
	Severity    Severity // neutral | warning | exhausted
	Stale       bool
}

type Snapshot struct {
	ProviderID    string
	ProviderName  string
	Status        Status
	Source        *Source // nil before first success
	Account       *Account
	Windows       []Window
	Balances      []Balance
	SpendCaps     []SpendCap
	Confidence    Confidence // exact | percent_only | estimated
	MeasuredAt    *time.Time // when the provider measured / we received accepted data
	CheckedAt     *time.Time // last attempt, success or failure
	NextRefreshAt *time.Time
	Refreshing    bool // a collection for this target is in flight
	Error         *UsageError
}

type Source struct {
	Kind  SourceKind // cli_rpc | oauth_http | api_key_http | acp_observed
	Label string     // "Codex app-server", "Claude Code OAuth", "OpenRouter API key"
}

type Account struct {
	Label string // email or org display name; display-only
	Plan  string // provider plan string, verbatim ("max_20x", "pro", "team")
}

type Window struct {
	ID              string     // stable per provider: "session", "weekly", "weekly_opus", "model:<slug>:weekly", "monthly"
	Label           string     // "Session", "Weekly", "Weekly · Opus"
	Kind            WindowKind // session | daily | weekly | monthly | model | other
	DurationSeconds *int64
	UsedPercent     *float64 // nil = unknown
	Used            *Amount  // optional raw counts when the provider reports them
	Limit           *Amount
	ResetsAt        *time.Time
	ResetText       string // provider text when no timestamp exists
}

type Amount struct {
	Value string // decimal string, no float rounding
	Unit  string // "USD", "CNY", "credits", "requests", "tokens"
}

type Balance struct {
	ID        string // "credits", "prepaid", "promo"
	Label     string
	Remaining Amount
	ExpiresAt *time.Time
}

type SpendCap struct {
	ID          string // "extra_usage", "key_limit", "monthly_budget"
	Label       string
	Used        Amount
	Limit       *Amount // nil = no cap reported (not "unlimited")
	UsedPercent *float64
	ResetsAt    *time.Time
	Enabled     *bool
}

type UsageError struct {
	Code       ErrorCode // see _dx.md Errors
	Message    string    // redacted, operator-readable
	Action     string    // "sign_in" | "allow_keychain" | "refresh" | "retry" | "configure" | ""
	RetryAfter *time.Time
}
