package cli

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func maybeApplyConfigSetViaDaemon(
	ctx context.Context,
	deps commandDeps,
	target compozyconfig.WriteTarget,
	path []string,
	value any,
	redacted bool,
) (*configSetRecord, error) {
	if !supportsDaemonManagedConfigSet(path, target) {
		return nil, nil
	}

	client, running, err := daemonClientIfRunning(ctx, deps)
	if err != nil {
		return nil, fmt.Errorf("cli: inspect daemon reachability for config set: %w", err)
	}
	if !running {
		return nil, nil
	}

	cfg, err := deps.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("cli: load current config for daemon-backed config set: %w", err)
	}

	result, err := applyDaemonManagedConfigValue(ctx, client, &cfg, path, value)
	if err != nil {
		return nil, fmt.Errorf("cli: apply %q via daemon settings surface: %w", strings.Join(path, "."), err)
	}

	outputValue := value
	if redacted {
		outputValue = compozyconfig.RedactedValue()
	}
	return &configSetRecord{
		Path:             strings.Join(path, "."),
		Value:            outputValue,
		Scope:            string(target.Scope()),
		Target:           target.Path(),
		Redacted:         redacted,
		Lifecycle:        string(result.Lifecycle),
		ApplyRecordID:    result.ApplyRecordID,
		Applied:          result.Applied,
		ActiveGeneration: result.ActiveGeneration,
		ActiveConfigHash: result.ActiveConfigHash,
		NextAction:       string(result.NextAction),
		RestartRequired:  result.RestartRequired,
		RestartScope:     result.RestartScope,
	}, nil
}

func applyDaemonManagedConfigValue(
	ctx context.Context,
	client DaemonClient,
	cfg *compozyconfig.Config,
	path []string,
	value any,
) (SettingsMutationRecord, error) {
	switch path[0] {
	case configSkillsKey:
		disabledSkills, ok := value.([]string)
		if !ok {
			return SettingsMutationRecord{}, fmt.Errorf(
				"cli: config set %q expects a string slice payload, got %T",
				strings.Join(path, "."),
				value,
			)
		}
		cfg.Skills.DisabledSkills = append([]string(nil), disabledSkills...)
		return client.UpdateSettingsSkills(ctx, UpdateSettingsSkillsRequest{
			Config: settingsSkillsPayloadFromConfig(cfg.Skills),
		})
	case configAttentionKey:
		if applyErr := applyAttentionConfigValue(&cfg.Attention, path, value); applyErr != nil {
			return SettingsMutationRecord{}, applyErr
		}
		return client.UpdateSettingsAttention(ctx, UpdateSettingsAttentionRequest{
			Config: settingsAttentionPayloadFromConfig(cfg.Attention),
		})
	case configShellKey:
		if applyErr := applyShellConfigValue(&cfg.Shell, path, value); applyErr != nil {
			return SettingsMutationRecord{}, applyErr
		}
		return client.UpdateSettingsShell(ctx, UpdateSettingsShellRequest{
			Config: settingsShellPayloadFromConfig(cfg.Shell),
		})
	default:
		return SettingsMutationRecord{}, fmt.Errorf(
			"cli: config set %q is not daemon-managed",
			strings.Join(path, "."),
		)
	}
}

func maybeReloadConfigAfterLocalWrite(
	ctx context.Context,
	deps commandDeps,
	target compozyconfig.WriteTarget,
	record configSetRecord,
) (*configSetRecord, error) {
	if target.Scope() != compozyconfig.WriteScopeUser {
		return nil, nil
	}
	client, running, err := daemonClientIfRunning(ctx, deps)
	if err != nil {
		return nil, fmt.Errorf("cli: inspect daemon reachability for config reload: %w", err)
	}
	if !running {
		return nil, nil
	}
	result, err := client.ReloadSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("cli: reload config via daemon settings surface: %w", err)
	}
	record.Lifecycle = string(result.Lifecycle)
	record.ApplyRecordID = result.ApplyRecordID
	record.Applied = result.Applied
	record.ActiveGeneration = result.ActiveGeneration
	record.ActiveConfigHash = result.ActiveConfigHash
	record.NextAction = string(result.NextAction)
	record.RestartRequired = result.RestartRequired
	record.RestartScope = result.RestartScope
	return &record, nil
}

func supportsDaemonManagedConfigSet(path []string, target compozyconfig.WriteTarget) bool {
	if target.Scope() != compozyconfig.WriteScopeUser {
		return false
	}
	if len(path) == 2 && path[0] == configSkillsKey && path[1] == agentDisabledSkillsKey {
		return true
	}
	return len(path) >= 2 && (path[0] == configAttentionKey || path[0] == configShellKey)
}

func settingsSkillsPayloadFromConfig(cfg compozyconfig.SkillsConfig) contract.SettingsSkillsConfigPayload {
	return contract.SettingsSkillsConfigPayload{
		Enabled:                 cfg.Enabled,
		DisabledSkills:          append([]string(nil), cfg.DisabledSkills...),
		PollInterval:            cfg.PollInterval.String(),
		AllowedMarketplaceMCP:   append([]string(nil), cfg.AllowedMarketplaceMCP...),
		AllowedMarketplaceHooks: append([]string(nil), cfg.AllowedMarketplaceHooks...),
		Marketplace: contract.SettingsMarketplacePayload{
			Registry: cfg.Marketplace.Registry,
			BaseURL:  cfg.Marketplace.BaseURL,
		},
	}
}

func configMutationPath(raw string) ([]string, configSetValueKind, bool, error) {
	segments, err := parseDottedConfigPath(raw)
	if err != nil {
		return nil, configSetString, false, err
	}
	if len(segments) > 3 && segments[0] == configWindowManagerKey &&
		(segments[1] == configWindowManagerShortcutsKey ||
			segments[1] == configWindowManagerGlobalShortcutsKey) {
		segments = []string{segments[0], segments[1], strings.Join(segments[2:], ".")}
	}
	kind, redacted, err := classifyConfigMutationPath(segments)
	if err != nil {
		return nil, configSetString, false, err
	}
	return segments, kind, redacted, nil
}

func parseDottedConfigPath(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("cli: config path is required")
	}
	if strings.ContainsAny(trimmed, "[]") {
		return nil, fmt.Errorf("cli: config set does not support array paths: %q", trimmed)
	}
	parts := strings.Split(trimmed, ".")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return nil, fmt.Errorf("cli: config path %q contains an empty segment", trimmed)
		}
	}
	return parts, nil
}
