package contract

// SettingsTerminalPayload projects the complete [terminal] policy block.
type SettingsTerminalPayload struct {
	DefaultShell           string `json:"default_shell"`
	ShellIntegration       bool   `json:"shell_integration"`
	ScrollbackBytes        int    `json:"scrollback_bytes"`
	DetachedTTL            string `json:"detached_ttl"`
	ExitRetention          string `json:"exit_retention"`
	Recording              bool   `json:"recording"`
	RecordingRetentionDays int    `json:"recording_retention_days"`
	MaxPerWorkspace        int    `json:"max_per_workspace"`
	MaxPerDaemon           int    `json:"max_per_daemon"`
	MaxSubscribers         int    `json:"max_subscribers"`
}
