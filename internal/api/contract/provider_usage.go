package contract

import "time"

type ProviderUsageEnvelope struct {
	KeychainSupported bool                     `json:"keychain_supported"`
	ProfileID         string                   `json:"profile_id"`
	Revision          uint64                   `json:"revision"`
	GeneratedAt       time.Time                `json:"generated_at"`
	Enabled           bool                     `json:"enabled"`
	Tray              *ProviderUsageTraySignal `json:"tray"`
	Targets           []ProviderUsageTarget    `json:"targets"`
}

type ProviderUsageTraySignal struct {
	ProviderID  string     `json:"provider_id"`
	WindowID    string     `json:"window_id"`
	Label       string     `json:"label"`
	UsedPercent float64    `json:"used_percent"`
	ResetsAt    *time.Time `json:"resets_at"`
	Severity    string     `json:"severity"`
	Stale       bool       `json:"stale"`
}

type ProviderUsageTarget struct {
	ProviderID    string                  `json:"provider_id"`
	ProviderName  string                  `json:"provider_name"`
	Status        string                  `json:"status"`
	Source        *ProviderUsageSource    `json:"source"`
	Account       *ProviderUsageAccount   `json:"account"`
	Windows       []ProviderUsageWindow   `json:"windows"`
	Balances      []ProviderUsageBalance  `json:"balances"`
	SpendCaps     []ProviderUsageSpendCap `json:"spend_caps"`
	Confidence    string                  `json:"confidence"`
	MeasuredAt    *time.Time              `json:"measured_at"`
	CheckedAt     *time.Time              `json:"checked_at"`
	NextRefreshAt *time.Time              `json:"next_refresh_at"`
	Refreshing    bool                    `json:"refreshing"`
	Error         *ProviderUsageError     `json:"error"`
}

type ProviderUsageSource struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type ProviderUsageAccount struct {
	Label string `json:"label"`
	Plan  string `json:"plan"`
}

type ProviderUsageWindow struct {
	ID              string               `json:"id"`
	Label           string               `json:"label"`
	Kind            string               `json:"kind"`
	DurationSeconds *int64               `json:"duration_seconds"`
	UsedPercent     *float64             `json:"used_percent"`
	Used            *ProviderUsageAmount `json:"used"`
	Limit           *ProviderUsageAmount `json:"limit"`
	ResetsAt        *time.Time           `json:"resets_at"`
	ResetText       string               `json:"reset_text"`
}

type ProviderUsageAmount struct {
	Value string `json:"value"`
	Unit  string `json:"unit"`
}

type ProviderUsageBalance struct {
	ID        string              `json:"id"`
	Label     string              `json:"label"`
	Remaining ProviderUsageAmount `json:"remaining"`
	ExpiresAt *time.Time          `json:"expires_at"`
}

type ProviderUsageSpendCap struct {
	ID          string               `json:"id"`
	Label       string               `json:"label"`
	Used        ProviderUsageAmount  `json:"used"`
	Limit       *ProviderUsageAmount `json:"limit"`
	UsedPercent *float64             `json:"used_percent"`
	ResetsAt    *time.Time           `json:"resets_at"`
	Enabled     *bool                `json:"enabled"`
}

type ProviderUsageError struct {
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	Action     string     `json:"action"`
	RetryAfter *time.Time `json:"retry_after"`
}

// ProviderUsageRefreshRequest selects one profile and bounded refresh wait.
type ProviderUsageRefreshRequest struct {
	Profile       string   `json:"profile,omitempty"`
	ProviderIDs   []string `json:"provider_ids,omitempty"`
	MaxAgeSeconds int64    `json:"max_age_seconds,omitempty"`
	WaitMS        *int64   `json:"wait_ms,omitempty"`
	Intent        string   `json:"intent,omitempty"`
}

type ProviderUsageStreamEvent struct {
	Revision uint64 `json:"revision"`
	BootID   string `json:"boot_id"`
}
