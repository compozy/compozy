package contract

import (
	"time"

	monitorpkg "github.com/compozy/compozy/internal/monitor"
)

// SessionMonitorSummaryPayload reports the session's monitors still waiting to
// wake it. A session whose turn ended but still has active monitors is not done.
type SessionMonitorSummaryPayload struct {
	Active int `json:"active"`
}

// SessionMonitorPayload is one daemon-owned monitor registered by a session.
type SessionMonitorPayload struct {
	MonitorID   string     `json:"monitor_id"`
	SessionID   string     `json:"session_id"`
	Kind        string     `json:"kind"`
	Description string     `json:"description"`
	TerminalID  string     `json:"terminal_id,omitempty"`
	Until       string     `json:"until,omitempty"`
	Pattern     string     `json:"pattern,omitempty"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	DeadlineAt  time.Time  `json:"deadline_at"`
	FiredAt     *time.Time `json:"fired_at,omitempty"`
	Reason      string     `json:"reason,omitempty"`
	ExitCode    *int       `json:"exit_code,omitempty"`
}

// SessionMonitorsResponse lists a session's monitors, active first, newest first.
type SessionMonitorsResponse struct {
	Monitors []SessionMonitorPayload `json:"monitors"`
}

// SessionMonitorResponse returns one monitor after a mutation.
type SessionMonitorResponse struct {
	Monitor SessionMonitorPayload `json:"monitor"`
}

// SessionMonitorSummaryFromCount omits the summary for a session with no active monitors.
func SessionMonitorSummaryFromCount(active int) *SessionMonitorSummaryPayload {
	if active <= 0 {
		return nil
	}
	return &SessionMonitorSummaryPayload{Active: active}
}

// SessionMonitorFromDomain maps one monitor to its wire payload.
func SessionMonitorFromDomain(item monitorpkg.Monitor) SessionMonitorPayload {
	payload := SessionMonitorPayload{
		MonitorID: item.ID, SessionID: item.SessionID, Kind: string(item.Kind),
		Description: item.Description, TerminalID: item.TerminalID, Until: item.Until,
		Pattern: item.Pattern, Status: string(item.Status), CreatedAt: item.CreatedAt,
		DeadlineAt: item.DeadlineAt, Reason: item.Reason, ExitCode: item.ExitCode,
	}
	if !item.FiredAt.IsZero() {
		payload.FiredAt = new(item.FiredAt)
	}
	return payload
}

// SessionMonitorsFromDomain maps a monitor list, never returning nil.
func SessionMonitorsFromDomain(items []monitorpkg.Monitor) []SessionMonitorPayload {
	payloads := make([]SessionMonitorPayload, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, SessionMonitorFromDomain(item))
	}
	return payloads
}
