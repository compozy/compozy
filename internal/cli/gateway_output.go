package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/spf13/cobra"
)

type gatewayProfileRecord struct {
	Name             string `json:"name"`
	Scheme           string `json:"scheme"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	DefaultWorkspace string `json:"default_workspace,omitempty"`
	RemoteHome       string `json:"remote_home,omitempty"`
	Active           bool   `json:"active"`
}

type gatewayProfileMutationRecord struct {
	Profile gatewayProfileRecord           `json:"profile"`
	Device  *contract.GatewayDevicePayload `json:"device,omitempty"`
	Status  string                         `json:"status"`
}

func gatewayStatusOutput(record contract.GatewayStatusPayload) outputBundle {
	return simpleGatewayOutput(record, func() string {
		return renderHumanSection("Gateway", []keyValue{
			{Label: "Enabled", Value: strconv.FormatBool(record.Enabled)},
			{Label: "Tiers", Value: strconv.Itoa(len(record.Tiers))},
			{Label: "Surfaces", Value: strconv.Itoa(len(record.Surfaces))},
			{Label: "Providers", Value: strconv.Itoa(len(record.Providers))},
			{Label: "Devices", Value: strconv.Itoa(len(record.Devices))},
		})
	}, func() string {
		return renderToonObject(clientTargetGateway, []string{
			extensionEnabledKey, "tiers", "surfaces", "providers", "devices", "bindings", windowManagerChangedField,
		}, []string{
			strconv.FormatBool(record.Enabled),
			strconv.Itoa(len(record.Tiers)),
			strconv.Itoa(len(record.Surfaces)),
			strconv.Itoa(len(record.Providers)),
			strconv.Itoa(len(record.Devices)),
			strconv.Itoa(len(record.Bindings)),
			strconv.FormatBool(record.Changed),
		})
	})
}

func gatewayAuditOutput(report contract.GatewayAuditPayload) outputBundle {
	return simpleGatewayOutput(report, func() string {
		result := "findings"
		if report.NoFindings {
			result = "no findings"
		}
		return renderHumanSection("Gateway audit", []keyValue{
			{Label: "Ran", Value: strconv.FormatBool(report.Ran)},
			{Label: "Result", Value: result},
			{Label: "Local only", Value: strconv.FormatBool(report.LocalOnly)},
			{Label: "Findings", Value: strconv.Itoa(len(report.Findings))},
			{Label: "Active devices", Value: strconv.Itoa(report.DeviceHighlights.Active)},
		})
	}, func() string {
		return renderToonObject("gateway_audit", []string{
			"ran", "no_findings", "local_only", "findings", "active_devices",
		}, []string{
			strconv.FormatBool(report.Ran),
			strconv.FormatBool(report.NoFindings),
			strconv.FormatBool(report.LocalOnly),
			strconv.Itoa(len(report.Findings)),
			strconv.Itoa(report.DeviceHighlights.Active),
		})
	})
}

func gatewayProfileFromConfig(
	profile compozyconfig.GatewayConnectionConfig,
	active string,
) gatewayProfileRecord {
	return gatewayProfileRecord{
		Name: profile.Name, Scheme: profile.Scheme, Host: profile.Host, Port: profile.Port,
		DefaultWorkspace: profile.DefaultWorkspace, RemoteHome: profile.RemoteHome,
		Active: strings.TrimSpace(active) == profile.Name,
	}
}

func gatewayProfileOutput(record gatewayProfileMutationRecord) outputBundle {
	return simpleGatewayOutput(record, func() string {
		return renderHumanSection("Connection", []keyValue{
			{Label: cliNameValue, Value: record.Profile.Name},
			{
				Label: automationTargetValue,
				Value: fmt.Sprintf("%s://%s:%d", record.Profile.Scheme, record.Profile.Host, record.Profile.Port),
			},
			{Label: automationStatusValue, Value: record.Status},
		})
	}, func() string {
		deviceID := ""
		if record.Device != nil {
			deviceID = record.Device.ID
		}
		return renderToonObject("connection", []string{
			automationNameKey,
			gatewayProfileTOMLSchemeKey,
			gatewayProfileTOMLHostKey,
			gatewayProfileTOMLPortKey,
			gatewayProfileTOMLDefaultWorkspaceKey,
			gatewayProfileTOMLRemoteHomeKey,
			memoryActiveKey,
			automationStatusKey,
			"device_id",
		}, []string{
			record.Profile.Name,
			record.Profile.Scheme,
			record.Profile.Host,
			strconv.Itoa(record.Profile.Port),
			record.Profile.DefaultWorkspace,
			record.Profile.RemoteHome,
			strconv.FormatBool(record.Profile.Active),
			record.Status,
			deviceID,
		})
	})
}

func gatewayProfilesOutput(records []gatewayProfileRecord) outputBundle {
	return listBundle(
		struct {
			Connections []gatewayProfileRecord `json:"connections"`
		}{Connections: records},
		records,
		"Connections",
		[]string{windowManagerNameHeader, "SCHEME", "HOST", "PORT", "ACTIVE"},
		"connections",
		[]string{
			automationNameKey,
			gatewayProfileTOMLSchemeKey,
			gatewayProfileTOMLHostKey,
			gatewayProfileTOMLPortKey,
			memoryActiveKey,
		},
		func(record gatewayProfileRecord) []string {
			return []string{
				record.Name, record.Scheme, record.Host, strconv.Itoa(record.Port), strconv.FormatBool(record.Active),
			}
		},
		func(record gatewayProfileRecord) []string {
			return []string{
				record.Name, record.Scheme, record.Host, strconv.Itoa(record.Port), strconv.FormatBool(record.Active),
			}
		},
	)
}

func gatewayDevicesOutput(records []contract.GatewayDevicePayload) outputBundle {
	return listBundle(
		contract.GatewayDevicesResponse{Devices: records},
		records,
		"Devices",
		[]string{"ID", windowManagerNameHeader, cliKindHeader, "REVOKED"},
		"devices",
		[]string{"id", automationNameKey, "actor_kind", "revoked"},
		func(record contract.GatewayDevicePayload) []string {
			return []string{record.ID, record.Name, record.ActorKind, strconv.FormatBool(record.RevokedAt != nil)}
		},
		func(record contract.GatewayDevicePayload) []string {
			return []string{record.ID, record.Name, record.ActorKind, strconv.FormatBool(record.RevokedAt != nil)}
		},
	)
}

func simpleGatewayOutput(value any, human func() string, toon func() string) outputBundle {
	return outputBundle{
		jsonValue: value,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, value)
		},
		human: func() (string, error) {
			return human(), nil
		},
		toon: func() (string, error) {
			return toon(), nil
		},
	}
}
