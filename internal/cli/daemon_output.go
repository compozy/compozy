package cli

import (
	"strconv"
	"strings"

	"time"

	"github.com/compozy/compozy/internal/api/contract"
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
	if status.Gateway != nil {
		gatewayFields := daemonGatewayStatusFields(status.Gateway)
		rows = append(rows, gatewayFields.rows...)
		labels = append(labels, gatewayFields.labels...)
		values = append(values, gatewayFields.values...)
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

type daemonGatewayOutputFields struct {
	rows   []keyValue
	labels []string
	values []string
}

func daemonGatewayStatusFields(info *contract.GatewayStatusPayload) daemonGatewayOutputFields {
	tiers := make([]string, 0, len(info.Tiers))
	for _, tier := range info.Tiers {
		listener := stringOrDash(tier.ListenerAddress)
		tiers = append(tiers, tier.Tier+":"+tier.Desired+"/"+tier.Observed+"@"+listener+
			" advertised="+strconv.FormatBool(tier.Advertised))
	}
	refusal := ""
	if info.Refusal != nil {
		refusal = strings.TrimSpace(info.Refusal.Cause)
		if fix := strings.TrimSpace(info.Refusal.Fix); fix != "" {
			refusal = strings.TrimSpace(refusal + " — " + fix)
		}
	}
	return daemonGatewayOutputFields{rows: []keyValue{
		{Label: "Gateway Enabled", Value: strconv.FormatBool(info.Enabled)},
		{Label: "Gateway Tiers", Value: stringOrDash(strings.Join(tiers, ", "))},
		{Label: "Gateway Surfaces", Value: strconv.Itoa(len(info.Surfaces))},
		{Label: "Gateway Providers", Value: strconv.Itoa(len(info.Providers))},
		{Label: "Gateway Addresses", Value: strconv.Itoa(len(info.Addresses))},
		{Label: "Gateway Devices", Value: strconv.Itoa(len(info.Devices))},
		{Label: "Gateway Refusal", Value: stringOrDash(refusal)},
	}, labels: []string{
		"gateway_enabled",
		"gateway_tiers",
		"gateway_surfaces",
		"gateway_providers",
		"gateway_addresses",
		"gateway_devices",
		"gateway_refusal",
	}, values: []string{
		strconv.FormatBool(info.Enabled),
		strings.Join(tiers, ", "),
		strconv.Itoa(len(info.Surfaces)),
		strconv.Itoa(len(info.Providers)),
		strconv.Itoa(len(info.Addresses)),
		strconv.Itoa(len(info.Devices)),
		refusal,
	}}
}
