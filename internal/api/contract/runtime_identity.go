package contract

import "time"

// RuntimeIdentityPayload is the bounded daemon identity used by local liveness clients.
type RuntimeIdentityPayload struct {
	SchemaVersion string                `json:"schema_version"`
	Daemon        DaemonIdentityPayload `json:"daemon"`
}

// DaemonIdentityPayload identifies one concrete daemon process and listener.
type DaemonIdentityPayload struct {
	PID         int       `json:"pid"`
	StartedAt   time.Time `json:"started_at"`
	HTTPHost    string    `json:"http_host"`
	HTTPPort    int       `json:"http_port"`
	UserHomeDir string    `json:"user_home_dir"`
	Version     string    `json:"version"`
}
