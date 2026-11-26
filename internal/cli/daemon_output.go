package cli

import (
	"strconv"
	"strings"

	"time"
)

func daemonStatusBundle(status DaemonStatus, now func() time.Time) outputBundle {
	rows := []keyValue{
		{Label: daemonStatusValue, Value: stringOrDash(status.Status)},
		{Label: cliPIDValue, Value: intOrDash(status.PID)},
		{Label: daemonStartedValue, Value: stringOrDash(formatTime(status.StartedAt))},
		{Label: cliUptimeValue, Value: stringOrDash(formatAge(now, status.StartedAt))},
		{Label: "Socket", Value: stringOrDash(status.Socket)},
		{Label: "HTTP", Value: stringOrDash(strings.TrimSpace(status.HTTPHost) + ":" + intOrDash(status.HTTPPort))},
		{Label: "Active Sessions", Value: strconv.Itoa(status.ActiveSessions)},
		{Label: "Total Sessions", Value: strconv.Itoa(status.TotalSessions)},
		{Label: versionValue, Value: stringOrDash(status.Version)},
	}
	labels := []string{
		daemonStatusKey,
		cliPIDKey,
		daemonStartedAtKey,
		"uptime",
		"socket",
		"http_host",
		"http_port",
		"active_sessions",
		"total_sessions",
		versionKey,
	}
	values := []string{
		status.Status,
		strconv.Itoa(status.PID),
		formatTime(status.StartedAt),
		formatAge(now, status.StartedAt),
		status.Socket,
		status.HTTPHost,
		strconv.Itoa(status.HTTPPort),
		strconv.Itoa(status.ActiveSessions),
		strconv.Itoa(status.TotalSessions),
		status.Version,
	}

	return outputBundle{
		jsonValue: status,
		human: func() (string, error) {
			return renderHumanSection("Daemon", rows), nil
		},
		toon: func() (string, error) {
			return renderToonObject(daemonDaemonKey, labels, values), nil
		},
	}
}
