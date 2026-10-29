package contract

import "time"

type SessionProcessPolicy string

const (
	SessionProcessesKeep      SessionProcessPolicy = "keep"
	SessionProcessesTerminate SessionProcessPolicy = "terminate"
)

type SessionProcessKind string

type SessionProcessOutcome string

type SessionProcessSummaryPayload struct {
	Supported  bool  `json:"supported"`
	Background int   `json:"background"`
	Terminals  int   `json:"terminals"`
	Total      int   `json:"total"`
	Ports      []int `json:"ports"`
}

type SessionProcessPayload struct {
	PID         int                `json:"pid"`
	PGID        int                `json:"pgid"`
	StartedAt   time.Time          `json:"started_at"`
	Kind        SessionProcessKind `json:"kind"`
	Command     string             `json:"command"`
	Argv0       string             `json:"argv0"`
	CWD         string             `json:"cwd"`
	Ports       []int              `json:"ports"`
	AgeSeconds  float64            `json:"age_seconds"`
	CPUPercent  *float64           `json:"cpu_percent,omitempty"`
	RSSKB       *int64             `json:"rss_kb,omitempty"`
	ParentAlive bool               `json:"parent_alive"`
	TerminalID  string             `json:"terminal_id,omitempty"`
}

type SessionProcessesResponse struct {
	Supported bool                    `json:"supported"`
	ScannedAt time.Time               `json:"scanned_at"`
	Items     []SessionProcessPayload `json:"items"`
}

type SessionProcessTarget struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

type TerminateSessionProcessesRequest struct {
	Targets []SessionProcessTarget `json:"targets,omitempty"`
	All     bool                   `json:"all,omitempty"`
}

type SessionProcessResult struct {
	PID       int                   `json:"pid"`
	StartedAt time.Time             `json:"started_at"`
	Outcome   SessionProcessOutcome `json:"outcome"`
	Error     string                `json:"error,omitempty"`
}

type TerminateSessionProcessesResponse struct {
	Supported bool                   `json:"supported"`
	Results   []SessionProcessResult `json:"results"`
}

type SessionProcessGroupPayload struct {
	SessionID   string                  `json:"session_id"`
	WorkspaceID string                  `json:"workspace_id"`
	Profile     string                  `json:"profile"`
	SessionName string                  `json:"session_name"`
	Orphaned    bool                    `json:"orphaned"`
	Items       []SessionProcessPayload `json:"items"`
}

type ProcessesResponse struct {
	Supported bool                         `json:"supported"`
	ScannedAt time.Time                    `json:"scanned_at"`
	Groups    []SessionProcessGroupPayload `json:"groups"`
}

type TerminateProcessesRequest struct {
	Targets []SessionProcessTarget `json:"targets"`
}
