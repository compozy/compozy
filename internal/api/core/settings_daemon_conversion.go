package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	aghconfig "github.com/compozy/compozy/internal/config"
)

func settingsDaemonPayload(value aghconfig.DaemonConfig) contract.SettingsDaemonPayload {
	return contract.SettingsDaemonPayload{
		Socket:               strings.TrimSpace(value.Socket),
		MemoryReportInterval: value.MemoryReportInterval.String(),
		ReloadTimeouts: contract.SettingsDaemonReloadTimeoutsPayload{
			Providers: value.ReloadTimeouts.Providers.String(),
			MCP:       value.ReloadTimeouts.MCP.String(),
			Bridges:   value.ReloadTimeouts.Bridges.String(),
		},
	}
}
