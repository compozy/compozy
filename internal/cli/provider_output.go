package cli

import (
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

func providerAuthStatusBundle(record providerAuthStatusRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			sections := []string{renderHumanSection("Provider Auth", providerAuthStatusRows(record))}
			if len(record.Credentials) > 0 {
				sections = append(sections, renderHumanTable(
					"Credentials",
					[]string{
						providerNameValue,
						"Target",
						authoredContextSourceValue,
						"Required",
						authoredContextPresentValue,
					},
					providerCredentialStatusRows(record.Credentials),
				))
			}
			return strings.Join(sections, "\n"), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"provider_auth",
				[]string{
					providerProviderKey,
					providerAuthModeKey,
					providerEnvPolicyKey,
					providerHomePolicyKey,
					providerStateKey,
				},
				[]string{record.Provider, record.AuthMode, record.EnvPolicy, record.HomePolicy, record.State},
			), nil
		},
	}
}

func providerAuthProbeBundle(record contract.ProviderAuthProbeResponse) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			rows := []keyValue{
				{Label: providerProviderValue, Value: stringOrDash(record.Provider)},
				{Label: providerAuthModeValue, Value: stringOrDash(record.AuthStatus.Mode)},
				{Label: "Env Policy", Value: stringOrDash(record.AuthStatus.EnvPolicy)},
				{Label: "Home Policy", Value: stringOrDash(record.AuthStatus.HomePolicy)},
				{Label: providerStateValue, Value: stringOrDash(record.AuthStatus.State)},
				{Label: cliCodeValue, Value: stringOrDash(record.AuthStatus.Code)},
				{Label: providerMessageValue, Value: stringOrDash(record.AuthStatus.Message)},
			}
			if record.Probe != nil {
				rows = append(rows,
					keyValue{Label: "Probe Exit Code", Value: fmt.Sprintf("%d", record.Probe.ExitCode)},
					keyValue{Label: "Probe Stdout", Value: stringOrDash(strings.TrimSpace(record.Probe.Stdout))},
					keyValue{Label: "Probe Stderr", Value: stringOrDash(strings.TrimSpace(record.Probe.Stderr))},
				)
			}
			return renderHumanSection("Provider Auth", rows), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"provider_auth",
				[]string{providerProviderKey, providerAuthModeKey, providerStateKey},
				[]string{record.Provider, record.AuthStatus.Mode, record.AuthStatus.State},
			), nil
		},
	}
}

func providerListBundle(record contract.ProviderListResponse) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			rows := make([][]string, 0, len(record.Providers))
			for _, provider := range record.Providers {
				rows = append(rows, []string{
					stringOrDash(provider.Name),
					stringOrDash(provider.DisplayName),
					stringOrDash(provider.AuthStatus.State),
					stringOrDash(provider.AuthStatus.Mode),
				})
			}
			return renderHumanTable(
				"Providers",
				[]string{providerNameValue, "Display Name", providerStateValue, providerAuthModeValue},
				rows,
			), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"providers",
				[]string{"count"},
				[]string{fmt.Sprintf("%d", len(record.Providers))},
			), nil
		},
	}
}

func providerAuthStatusRows(record providerAuthStatusRecord) []keyValue {
	rows := []keyValue{
		{Label: providerProviderValue, Value: stringOrDash(record.Provider)},
		{Label: "Display Name", Value: stringOrDash(record.DisplayName)},
		{Label: providerAuthModeValue, Value: stringOrDash(record.AuthMode)},
		{Label: "Env Policy", Value: stringOrDash(record.EnvPolicy)},
		{Label: "Home Policy", Value: stringOrDash(record.HomePolicy)},
		{Label: providerStateValue, Value: stringOrDash(record.State)},
		{Label: cliCodeValue, Value: stringOrDash(record.Code)},
		{Label: providerMessageValue, Value: stringOrDash(record.Message)},
		{Label: "Status Command", Value: stringOrDash(record.StatusCommand)},
		{Label: "Login Command", Value: stringOrDash(record.LoginCommand)},
	}
	if len(record.LoginEnv) > 0 {
		rows = append(rows, keyValue{Label: "Login Env", Value: strings.Join(record.LoginEnv, " ")})
	}
	if record.NativeCLI != nil {
		rows = append(rows,
			keyValue{Label: "Native CLI Command", Value: stringOrDash(record.NativeCLI.Command)},
			keyValue{Label: "Native CLI Present", Value: boolString(record.NativeCLI.Present)},
			keyValue{Label: "Native CLI Path", Value: stringOrDash(record.NativeCLI.Path)},
			keyValue{Label: "Native CLI Source", Value: stringOrDash(record.NativeCLI.Source)},
		)
		if record.NativeCLI.Error != "" {
			rows = append(rows, keyValue{Label: "Native CLI Error", Value: record.NativeCLI.Error})
		}
	}
	if record.Probe != nil {
		rows = append(rows,
			keyValue{Label: "Probe Exit Code", Value: fmt.Sprintf("%d", record.Probe.ExitCode)},
			keyValue{Label: "Probe Stdout", Value: stringOrDash(strings.TrimSpace(record.Probe.Stdout))},
			keyValue{Label: "Probe Stderr", Value: stringOrDash(strings.TrimSpace(record.Probe.Stderr))},
		)
	}
	return rows
}

func providerCredentialStatusRows(statuses []providerCredentialStatusItem) [][]string {
	rows := make([][]string, 0, len(statuses))
	for _, status := range statuses {
		rows = append(rows, []string{
			stringOrDash(status.Name),
			stringOrDash(status.TargetEnv),
			stringOrDash(status.Source),
			boolString(status.Required),
			boolString(status.Present),
		})
	}
	return rows
}
