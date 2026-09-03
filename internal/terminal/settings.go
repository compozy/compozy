package terminal

import "time"

type Settings struct {
	DefaultShell           string
	ShellIntegration       bool
	ScrollbackBytes        int
	DetachedTTL            time.Duration
	ExitRetention          time.Duration
	Recording              bool
	RecordingRetentionDays int
	MaxPerWorkspace        int
	MaxPerDaemon           int
	MaxSubscribers         int
}

func DefaultSettings() Settings {
	return Settings{
		ShellIntegration:       true,
		ScrollbackBytes:        1 << 20,
		DetachedTTL:            24 * time.Hour,
		ExitRetention:          15 * time.Minute,
		RecordingRetentionDays: 30,
		MaxPerWorkspace:        8,
		MaxPerDaemon:           32,
		MaxSubscribers:         16,
	}
}
