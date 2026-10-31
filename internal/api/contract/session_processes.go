package contract

import "time"

type SessionProcessPolicy string

const (
	SessionProcessesKeep      SessionProcessPolicy = "keep"
	SessionProcessesTerminate SessionProcessPolicy = "terminate"
)

type SessionProcessAttribution string

type SessionProcessKind string

type SessionProcessOutcome string

type SessionProcessSummaryPayload struct {
	Supported  bool  `json:"supported"`
	Background int   `json:"background"`
	Terminals  int   `json:"terminals"`
	Total      int   `json:"total"`
	Ports      []int `json:"ports"`
	// Trees counts the Background and Terminal process trees the session left running; the
	// per-process background and terminals counts stay for compatibility.
	Trees int `json:"trees,omitempty"`
}

type SessionProcessPayload struct {
	Attribution SessionProcessAttribution `json:"attribution"`
	PID         int                       `json:"pid"`
	PGID        int                       `json:"pgid"`
	StartedAt   time.Time                 `json:"started_at"`
	Kind        SessionProcessKind        `json:"kind"`
	Command     string                    `json:"command"`
	Argv0       string                    `json:"argv0"`
	CWD         string                    `json:"cwd"`
	Ports       []int                     `json:"ports"`
	AgeSeconds  float64                   `json:"age_seconds"`
	CPUPercent  *float64                  `json:"cpu_percent,omitempty"`
	RSSKB       *int64                    `json:"rss_kb,omitempty"`
	ParentAlive bool                      `json:"parent_alive"`
	TerminalID  string                    `json:"terminal_id,omitempty"`
	// TreeID names the process tree this process belongs to; see SessionProcessTreePayload.ID.
	TreeID string `json:"tree_id,omitempty"`
}

// SessionProcessTreePayload is one logical thing a session started: its root, the lead process
// that names it, the wrappers that launched the lead, and every member a stop signals.
type SessionProcessTreePayload struct {
	ID       string                 `json:"id"`
	Kind     SessionProcessKind     `json:"kind"`
	Root     SessionProcessTarget   `json:"root"`
	Lead     SessionProcessTarget   `json:"lead"`
	Wrappers []SessionProcessTarget `json:"wrappers"`
	Members  []SessionProcessTarget `json:"members"`
	Ports    []int                  `json:"ports"`
}

type SessionProcessesResponse struct {
	Supported bool                        `json:"supported"`
	ScannedAt time.Time                   `json:"scanned_at"`
	Items     []SessionProcessPayload     `json:"items"`
	Trees     []SessionProcessTreePayload `json:"trees,omitempty"`
}

type SessionProcessTarget struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

// TerminateSessionProcessesRequest stops each target's whole process tree. All stops every tree the
// session left running; tool calls still running in the agent's turn join only with IncludeTools.
type TerminateSessionProcessesRequest struct {
	Targets      []SessionProcessTarget `json:"targets,omitempty"`
	All          bool                   `json:"all,omitempty"`
	IncludeTools bool                   `json:"include_tools,omitempty"`
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
	SessionID   string                      `json:"session_id"`
	WorkspaceID string                      `json:"workspace_id"`
	Profile     string                      `json:"profile"`
	SessionName string                      `json:"session_name"`
	Orphaned    bool                        `json:"orphaned"`
	Items       []SessionProcessPayload     `json:"items"`
	Trees       []SessionProcessTreePayload `json:"trees,omitempty"`
}

type ProcessesResponse struct {
	Supported bool                         `json:"supported"`
	ScannedAt time.Time                    `json:"scanned_at"`
	Groups    []SessionProcessGroupPayload `json:"groups"`
}

type TerminateProcessesRequest struct {
	Targets []SessionProcessTarget `json:"targets"`
}
