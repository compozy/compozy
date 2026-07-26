package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

func configShowBundle(record configShowRecord, entries []configEntry) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderConfigEntries("Config", entries), nil
		},
		toon: func() (string, error) {
			return renderConfigEntriesToon(configConfigKey, entries), nil
		},
	}
}

func configListBundle(record configListRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderConfigEntries("Config", record.Entries), nil
		},
		toon: func() (string, error) {
			return renderConfigEntriesToon(configConfigKey, record.Entries), nil
		},
	}
}

func configValueBundle(record configValueRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return fmt.Sprintf("%s: %s", record.Path, formatConfigValue(record.Value)), nil
		},
		toon: func() (string, error) {
			return renderToonObject("config_value", []string{configPathKey, hooksValueKey, configRedactedKey}, []string{
				record.Path,
				formatConfigValue(record.Value),
				strconv.FormatBool(record.Redacted),
			}), nil
		},
	}
}

func configSetBundle(record configSetRecord) outputBundle {
	rows := []keyValue{
		{Label: configPathValue, Value: stringOrDash(record.Path)},
		{Label: configValueValue, Value: formatConfigValue(record.Value)},
		{Label: configScopeValue, Value: stringOrDash(record.Scope)},
		{Label: configTargetValue, Value: stringOrDash(record.Target)},
		{Label: configRedactedValue, Value: strconv.FormatBool(record.Redacted)},
		{Label: cliLifecycleValue, Value: stringOrDash(record.Lifecycle)},
		{Label: cliAppliedValue, Value: strconv.FormatBool(record.Applied)},
		{Label: cliNextActionValue, Value: stringOrDash(record.NextAction)},
		{Label: "Apply Record", Value: stringOrDash(record.ApplyRecordID)},
		{Label: cliActiveGenerationValue, Value: strconv.FormatInt(record.ActiveGeneration, 10)},
		{Label: "Restart Required", Value: strconv.FormatBool(record.RestartRequired)},
		{Label: "Restart Scope", Value: stringOrDash(record.RestartScope)},
	}
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Config", rows), nil
		},
		toon: func() (string, error) {
			return renderToonObject("config_set", []string{
				configPathKey,
				hooksValueKey,
				configScopeKey,
				configTargetKey,
				configRedactedKey,
				cliLifecycleKey,
				cliAppliedKey,
				cliNextActionKey,
				cliApplyRecordIDKey,
				cliActiveGenerationKey,
				"restart_required",
				"restart_scope",
			}, []string{
				record.Path,
				formatConfigValue(record.Value),
				record.Scope,
				record.Target,
				strconv.FormatBool(record.Redacted),
				record.Lifecycle,
				strconv.FormatBool(record.Applied),
				record.NextAction,
				record.ApplyRecordID,
				strconv.FormatInt(record.ActiveGeneration, 10),
				strconv.FormatBool(record.RestartRequired),
				record.RestartScope,
			}), nil
		},
	}
}

func configApplyHistoryBundle(record SettingsApplyHistoryRecord) outputBundle {
	rows := make([][]string, 0, len(record.Entries))
	for _, entry := range record.Entries {
		rows = append(rows, []string{
			entry.ID,
			string(entry.Status),
			string(entry.Lifecycle),
			strconv.FormatInt(entry.Generation, 10),
			entry.Actor,
			string(entry.NextAction),
			entry.UpdatedAt.Format(time.RFC3339),
		})
	}
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanTable("Config Apply History", []string{
				"ID",
				configStatusValue,
				cliLifecycleValue,
				"Generation",
				"Actor",
				cliNextActionValue,
				"Updated",
			}, rows), nil
		},
		toon: func() (string, error) {
			data, err := json.Marshal(record)
			if err != nil {
				return "", fmt.Errorf("cli: marshal config apply-history toon payload: %w", err)
			}
			return renderToonObject("config_apply_history", []string{"entries"}, []string{string(data)}), nil
		},
	}
}

func configPathBundle(record configPathRecord) outputBundle {
	rows := []keyValue{
		{Label: "Home", Value: stringOrDash(record.HomeDir)},
		{Label: "Global Config", Value: stringOrDash(record.GlobalConfig)},
		{Label: "Global MCP JSON", Value: stringOrDash(record.GlobalMCPJSON)},
		{Label: configScopeValue, Value: stringOrDash(record.Scope)},
		{Label: "Selected Config Target", Value: stringOrDash(record.SelectedConfigTarget)},
		{Label: configManagedValue, Value: strconv.FormatBool(record.Managed)},
		{Label: configManagerValue, Value: stringOrDash(record.Manager)},
	}
	if record.WorkspaceRoot != "" {
		rows = append(rows,
			keyValue{Label: configWorkspaceValue, Value: record.WorkspaceRoot},
			keyValue{Label: "Workspace Config", Value: record.WorkspaceConfig},
			keyValue{Label: "Workspace MCP JSON", Value: record.WorkspaceMCPJSON},
		)
	}
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Config Paths", rows), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"config_paths",
				[]string{
					"home_dir",
					"global_config",
					"global_mcp_json",
					configScopeKey,
					configWorkspaceRootKey,
					"selected_config_target",
					configManagedKey,
					configManagerKey,
				},
				[]string{
					record.HomeDir,
					record.GlobalConfig,
					record.GlobalMCPJSON,
					record.Scope,
					record.WorkspaceRoot,
					record.SelectedConfigTarget,
					strconv.FormatBool(record.Managed),
					record.Manager,
				},
			), nil
		},
	}
}

func configValidateBundle(record configValidateRecord) outputBundle {
	rows := []keyValue{
		{Label: configStatusValue, Value: stringOrDash(record.Status)},
		{Label: configScopeValue, Value: stringOrDash(record.Scope)},
		{Label: configWorkspaceValue, Value: stringOrDash(record.WorkspaceRoot)},
		{Label: "Config File", Value: stringOrDash(record.ConfigFile)},
		{Label: configRedactedValue, Value: strconv.FormatBool(record.Redacted)},
	}
	if record.DotEnv != nil {
		rows = append(rows,
			keyValue{Label: ".env Path", Value: stringOrDash(record.DotEnv.Path)},
			keyValue{Label: ".env Status", Value: stringOrDash(record.DotEnv.Status)},
			keyValue{Label: ".env Repaired", Value: strconv.FormatBool(record.DotEnv.Repaired)},
		)
		if len(record.DotEnv.Diagnostics) > 0 {
			rows = append(rows, keyValue{
				Label: ".env Diagnostics",
				Value: strings.Join(dotEnvDiagnosticSummaries(record.DotEnv.Diagnostics), "; "),
			})
		}
	}
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Config Validation", rows), nil
		},
		toon: func() (string, error) {
			fields := []string{
				configStatusKey,
				configScopeKey,
				configWorkspaceRootKey,
				"config_file",
				configRedactedKey,
			}
			values := []string{
				record.Status,
				record.Scope,
				record.WorkspaceRoot,
				record.ConfigFile,
				strconv.FormatBool(record.Redacted),
			}
			if record.DotEnv != nil {
				fields = append(fields, "dot_env_status", "dot_env_repaired")
				values = append(values, record.DotEnv.Status, strconv.FormatBool(record.DotEnv.Repaired))
			}
			return renderToonObject("config_validation", fields, values), nil
		},
	}
}

func dotEnvDiagnosticSummaries(diagnostics []aghconfig.DotEnvDiagnostic) []string {
	summaries := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		location := ""
		if diagnostic.Line > 0 {
			location = fmt.Sprintf("line %d", diagnostic.Line)
		}
		if diagnostic.Key != "" {
			if location != "" {
				location += " "
			}
			location += diagnostic.Key
		}
		if location == "" {
			location = "file"
		}
		summaries = append(summaries, location+": "+diagnostic.Message)
	}
	return summaries
}

func renderConfigEntries(title string, entries []configEntry) string {
	return renderHumanTable(
		title,
		[]string{configPathValue, configValueValue, configRedactedValue},
		configEntryRows(entries),
	)
}

func configEntryRows(entries []configEntry) [][]string {
	rows := make([][]string, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, []string{
			entry.Path,
			formatConfigValue(entry.Value),
			strconv.FormatBool(entry.Redacted),
		})
	}
	return rows
}

func renderConfigEntriesToon(name string, entries []configEntry) string {
	rows := make([][]string, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, []string{entry.Path, formatConfigValue(entry.Value), strconv.FormatBool(entry.Redacted)})
	}
	return renderToonArray(name, []string{configPathKey, hooksValueKey, configRedactedKey}, rows)
}

func formatConfigValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		payload, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return compactJSON(payload)
	}
}
