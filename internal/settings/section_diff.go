package settings

import (
	"maps"
	"reflect"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func diffSkillsSettings(current compozyconfig.SkillsConfig, desired compozyconfig.SkillsConfig) []string {
	var changed []string
	if current.Enabled != desired.Enabled {
		changed = append(changed, "skills.enabled")
	}
	if !reflect.DeepEqual(current.Sources, desired.Sources) {
		changed = append(changed, "skills.sources")
	}
	if !reflect.DeepEqual(current.CustomSources, desired.CustomSources) {
		changed = append(changed, "skills.custom_sources")
	}
	if !reflect.DeepEqual(current.DisabledSkills, desired.DisabledSkills) {
		changed = append(changed, "skills.disabled_skills")
	}
	if current.PollInterval != desired.PollInterval {
		changed = append(changed, "skills.poll_interval")
	}
	if !reflect.DeepEqual(current.AllowedMarketplaceHooks, desired.AllowedMarketplaceHooks) {
		changed = append(changed, "skills.allowed_marketplace_hooks")
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

// diffObservabilityOTelSettings lists [observability.otel] keys, which the Settings form does not edit.
func diffObservabilityOTelSettings(
	current compozyconfig.ObservabilityOTelConfig,
	desired compozyconfig.ObservabilityOTelConfig,
) []string {
	var changed []string
	if current.Enabled != desired.Enabled {
		changed = append(changed, "observability.otel.enabled")
	}
	if current.Protocol != desired.Protocol {
		changed = append(changed, "observability.otel.protocol")
	}
	if current.Endpoint != desired.Endpoint {
		changed = append(changed, "observability.otel.endpoint")
	}
	if !maps.Equal(current.Headers, desired.Headers) {
		changed = append(changed, "observability.otel.headers")
	}
	if current.Traces != desired.Traces {
		changed = append(changed, "observability.otel.traces")
	}
	if current.Metrics != desired.Metrics {
		changed = append(changed, "observability.otel.metrics")
	}
	if current.SampleRatio != desired.SampleRatio {
		changed = append(changed, "observability.otel.sample_ratio")
	}
	if current.MetricInterval != desired.MetricInterval {
		changed = append(changed, "observability.otel.metric_interval")
	}
	if current.ServiceName != desired.ServiceName {
		changed = append(changed, "observability.otel.service_name")
	}
	if current.SlowDBWriteMS != desired.SlowDBWriteMS {
		changed = append(changed, "observability.otel.slow_db_write_ms")
	}
	if current.ForwardToProviders != desired.ForwardToProviders {
		changed = append(changed, "observability.otel.forward_to_providers")
	}
	return changed
}
