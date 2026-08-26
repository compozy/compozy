package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/spf13/cobra"
)

func maybeApplyConfigSetViaDaemon(
	cmd *cobra.Command,
	deps commandDeps,
	target compozyconfig.WriteTarget,
	workspaceRef string,
	path []string,
	value any,
	redacted bool,
) (*configSetRecord, error) {
	if !supportsDaemonManagedConfigSet(path, target) {
		return nil, nil
	}

	client, running, err := daemonClientIfRunning(cmd.Context(), deps)
	if err != nil {
		return nil, fmt.Errorf("cli: inspect daemon reachability for config set: %w", err)
	}
	if !running {
		return nil, nil
	}

	var result SettingsMutationRecord
	if isSkillSourceMutation(path) {
		result, err = applyDaemonSkillSourceValue(cmd, deps, client, target, workspaceRef, path, value)
	} else {
		cfg, loadErr := deps.loadConfig()
		if loadErr != nil {
			return nil, fmt.Errorf("cli: load current config for daemon-backed config set: %w", loadErr)
		}
		result, err = applyDaemonManagedConfigValue(cmd.Context(), client, &cfg, path, value)
	}
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

func isSkillSourceMutation(path []string) bool {
	return len(path) == 2 && path[0] == configSkillsKey &&
		(path[1] == skillSourcesField || path[1] == skillCustomSourcesField)
}

func applyDaemonSkillSourceValue(
	cmd *cobra.Command,
	deps commandDeps,
	client DaemonClient,
	target compozyconfig.WriteTarget,
	workspaceRef string,
	path []string,
	value any,
) (SettingsMutationRecord, error) {
	values, ok := value.([]string)
	if !ok {
		return SettingsMutationRecord{}, fmt.Errorf(
			"cli: config set %q expects a string slice payload, got %T",
			strings.Join(path, "."), value,
		)
	}
	if target.Scope() == compozyconfig.WriteScopeWorkspace || target.Scope() == compozyconfig.WriteScopeProfile {
		query := settingsSkillsScopeQuery{Scope: contract.SettingsScopeWorkspace}
		if target.Scope() == compozyconfig.WriteScopeProfile {
			profileName, err := resolveConfigWriteProfile(cmd, deps)
			if err != nil {
				return SettingsMutationRecord{}, err
			}
			query = settingsSkillsScopeQuery{Scope: contract.SettingsScopeProfile, Profile: profileName}
		} else {
			resolution, err := resolveCommandWorkspace(
				cmd.Context(), cmd, deps, client, workspaceResolutionRequest{FlagRef: workspaceRef},
			)
			if err != nil {
				return SettingsMutationRecord{}, err
			}
			query.WorkspaceID = resolution.ID
		}
		override := contract.SettingsSkillsOverridePayload{}
		optional := contract.OptionalStringList{Present: true, Value: append([]string{}, values...)}
		if path[1] == skillSourcesField {
			override.Sources = optional
		} else {
			override.CustomSources = optional
		}
		return updateSettingsSkillsAtScope(
			cmd.Context(),
			client,
			query,
			UpdateSettingsSkillsRequest{Override: &override},
		)
	}

	scope := contract.SettingsScopeUser
	scopeRaw := string(compozyconfig.WriteScopeUser)
	query := settingsSkillsScopeQuery{Scope: scope}
	cfg, err := loadConfigForDisplayScope(cmd, deps, scopeRaw, workspaceRef)
	if err != nil {
		return SettingsMutationRecord{}, fmt.Errorf("cli: load current skill source config: %w", err)
	}
	if path[1] == skillSourcesField {
		cfg.Skills.Sources = append([]string{}, values...)
	} else {
		cfg.Skills.CustomSources = append([]string{}, values...)
	}
	if err := cfg.Skills.ValidateForScope(target.Scope()); err != nil {
		return SettingsMutationRecord{}, err
	}
	request := UpdateSettingsSkillsRequest{Config: settingsSkillsPayloadFromConfig(cfg.Skills)}
	if target.Scope() == compozyconfig.WriteScopeUser {
		return client.UpdateSettingsSkills(cmd.Context(), request)
	}
	return updateSettingsSkillsAtScope(cmd.Context(), client, query, request)
}

func updateSettingsSkillsAtScope(
	ctx context.Context,
	client DaemonClient,
	query settingsSkillsScopeQuery,
	request UpdateSettingsSkillsRequest,
) (SettingsMutationRecord, error) {
	scoped, ok := client.(scopedSettingsSkillsClient)
	if !ok {
		return SettingsMutationRecord{}, errors.New("cli: daemon client does not support scoped skills settings")
	}
	return scoped.UpdateSettingsSkillsAtScope(ctx, query, request)
}

func maybeUnsetWorkspaceSkillSourceViaDaemon(
	cmd *cobra.Command,
	deps commandDeps,
	target compozyconfig.WriteTarget,
	workspaceRef string,
	path []string,
) (*configUnsetRecord, error) {
	if (target.Scope() != compozyconfig.WriteScopeWorkspace &&
		target.Scope() != compozyconfig.WriteScopeProfile) || !isSkillSourceMutation(path) {
		return nil, nil
	}
	client, running, err := daemonClientIfRunning(cmd.Context(), deps)
	if err != nil {
		return nil, fmt.Errorf("cli: inspect daemon reachability for config unset: %w", err)
	}
	if !running {
		return nil, nil
	}
	query := settingsSkillsScopeQuery{Scope: contract.SettingsScopeWorkspace}
	if target.Scope() == compozyconfig.WriteScopeProfile {
		profileName, resolveErr := resolveConfigWriteProfile(cmd, deps)
		if resolveErr != nil {
			return nil, resolveErr
		}
		query = settingsSkillsScopeQuery{Scope: contract.SettingsScopeProfile, Profile: profileName}
	} else {
		resolution, resolveErr := resolveCommandWorkspace(
			cmd.Context(), cmd, deps, client, workspaceResolutionRequest{FlagRef: workspaceRef},
		)
		if resolveErr != nil {
			return nil, resolveErr
		}
		query.WorkspaceID = resolution.ID
	}
	override := contract.SettingsSkillsOverridePayload{}
	optional := contract.OptionalStringList{Present: true, Null: true}
	if path[1] == skillSourcesField {
		override.Sources = optional
	} else {
		override.CustomSources = optional
	}
	result, err := updateSettingsSkillsAtScope(
		cmd.Context(),
		client,
		query,
		UpdateSettingsSkillsRequest{Override: &override},
	)
	if err != nil {
		return nil, fmt.Errorf("cli: unset %q via daemon settings surface: %w", strings.Join(path, "."), err)
	}
	return &configUnsetRecord{
		Path:             strings.Join(path, "."),
		Scope:            string(target.Scope()),
		Target:           target.Path(),
		Deleted:          true,
		Lifecycle:        string(result.Lifecycle),
		ApplyRecordID:    result.ApplyRecordID,
		Applied:          result.Applied,
		ActiveGeneration: result.ActiveGeneration,
		ActiveConfigHash: result.ActiveConfigHash,
		NextAction: string(
			result.NextAction,
		),
		RestartRequired: result.RestartRequired,
		RestartScope:    result.RestartScope,
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
		values, ok := value.([]string)
		if !ok {
			return SettingsMutationRecord{}, fmt.Errorf(
				"cli: config set %q expects a string slice payload, got %T",
				strings.Join(path, "."),
				value,
			)
		}
		switch path[1] {
		case skillSourcesField:
			cfg.Skills.Sources = append([]string(nil), values...)
		case "custom_sources":
			cfg.Skills.CustomSources = append([]string(nil), values...)
		case agentDisabledSkillsKey:
			cfg.Skills.DisabledSkills = append([]string(nil), values...)
		default:
			return SettingsMutationRecord{}, fmt.Errorf(
				"cli: config set %q is not daemon-managed",
				strings.Join(path, "."),
			)
		}
		if err := cfg.Skills.ValidateForScope(compozyconfig.WriteScopeUser); err != nil {
			return SettingsMutationRecord{}, err
		}
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
	record.ActiveGeneration = result.ActiveGeneration
	record.ActiveConfigHash = result.ActiveConfigHash
	if record.Status != configStatusOverridden {
		record.Applied = result.Applied
		record.NextAction = string(result.NextAction)
	}
	record.RestartRequired = result.RestartRequired
	record.RestartScope = result.RestartScope
	return &record, nil
}

func supportsDaemonManagedConfigSet(path []string, target compozyconfig.WriteTarget) bool {
	if len(path) == 2 && path[0] == configSkillsKey {
		switch path[1] {
		case skillSourcesField, skillCustomSourcesField:
			return true
		case agentDisabledSkillsKey:
			return target.Scope() == compozyconfig.WriteScopeUser
		}
	}
	if target.Scope() != compozyconfig.WriteScopeUser {
		return false
	}
	return len(path) >= 2 && (path[0] == configAttentionKey || path[0] == configShellKey)
}

func settingsSkillsPayloadFromConfig(cfg compozyconfig.SkillsConfig) contract.SettingsSkillsConfigPayload {
	return contract.SettingsSkillsConfigPayload{
		Enabled:                 cfg.Enabled,
		Sources:                 append([]string(nil), cfg.Sources...),
		CustomSources:           append([]string(nil), cfg.CustomSources...),
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
