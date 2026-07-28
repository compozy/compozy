package settings

import (
	"reflect"

	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func diffMemorySettings(current *compozyconfig.MemoryConfig, desired *compozyconfig.MemoryConfig) []string {
	var changed []string
	currentValues := memorySettingsUpdates(current)
	desiredValues := memorySettingsUpdates(desired)
	for i, currentValue := range currentValues {
		if i >= len(desiredValues) {
			break
		}
		desiredValue := desiredValues[i]
		if reflect.DeepEqual(currentValue.value, desiredValue.value) {
			continue
		}
		changed = append(changed, strings.Join(currentValue.path, "."))
	}
	return changed
}

func diffSkillsSettings(current compozyconfig.SkillsConfig, desired compozyconfig.SkillsConfig) []string {
	var changed []string
	if current.Enabled != desired.Enabled {
		changed = append(changed, "skills.enabled")
	}
	if !reflect.DeepEqual(current.DisabledSkills, desired.DisabledSkills) {
		changed = append(changed, "skills.disabled_skills")
	}
	if current.PollInterval != desired.PollInterval {
		changed = append(changed, "skills.poll_interval")
	}
	if !reflect.DeepEqual(current.AllowedMarketplaceMCP, desired.AllowedMarketplaceMCP) {
		changed = append(changed, "skills.allowed_marketplace_mcp")
	}
	if !reflect.DeepEqual(current.AllowedMarketplaceHooks, desired.AllowedMarketplaceHooks) {
		changed = append(changed, "skills.allowed_marketplace_hooks")
	}
	if current.Marketplace.Registry != desired.Marketplace.Registry {
		changed = append(changed, "skills.marketplace.registry")
	}
	if current.Marketplace.BaseURL != desired.Marketplace.BaseURL {
		changed = append(changed, "skills.marketplace.base_url")
	}
	return changed
}

func diffAgentImmutableSkillsSettings(current compozyconfig.SkillsConfig, desired compozyconfig.SkillsConfig) []string {
	baseCurrent := current
	baseCurrent.DisabledSkills = nil
	baseDesired := desired
	baseDesired.DisabledSkills = nil
	return diffSkillsSettings(baseCurrent, baseDesired)
}

func diffAutomationSettings(cfg *compozyconfig.Config, desired AutomationSettings) []string {
	var changed []string
	if cfg.Automation.Enabled != desired.Enabled {
		changed = append(changed, "automation.enabled")
	}
	if cfg.Automation.Timezone != desired.Timezone {
		changed = append(changed, "automation.timezone")
	}
	if cfg.Automation.MaxConcurrentJobs != desired.MaxConcurrentJobs {
		changed = append(changed, "automation.max_concurrent_jobs")
	}
	if cfg.Automation.DefaultFireLimit != desired.DefaultFireLimit {
		changed = append(changed, "automation.default_fire_limit")
	}
	return changed
}

func diffObservabilitySettings(
	current compozyconfig.ObservabilityConfig,
	desired compozyconfig.ObservabilityConfig,
) []string {
	var changed []string
	if current.Enabled != desired.Enabled {
		changed = append(changed, "observability.enabled")
	}
	if current.RetentionDays != desired.RetentionDays {
		changed = append(changed, "observability.retention_days")
	}
	if current.MaxGlobalBytes != desired.MaxGlobalBytes {
		changed = append(changed, "observability.max_global_bytes")
	}
	if current.Transcripts.Enabled != desired.Transcripts.Enabled {
		changed = append(changed, "observability.transcripts.enabled")
	}
	if current.Transcripts.SegmentBytes != desired.Transcripts.SegmentBytes {
		changed = append(changed, "observability.transcripts.segment_bytes")
	}
	if current.Transcripts.MaxBytesPerSession != desired.Transcripts.MaxBytesPerSession {
		changed = append(changed, "observability.transcripts.max_bytes_per_session")
	}
	return changed
}
