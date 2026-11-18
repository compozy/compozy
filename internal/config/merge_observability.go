package config

import (
	"maps"
	"time"
)

type observabilityOTelOverlay struct {
	Enabled            *bool             `toml:"enabled"`
	Protocol           *string           `toml:"protocol"`
	Endpoint           *string           `toml:"endpoint"`
	Headers            map[string]string `toml:"headers"`
	Traces             *bool             `toml:"traces"`
	Metrics            *bool             `toml:"metrics"`
	SampleRatio        *float64          `toml:"sample_ratio"`
	MetricInterval     *time.Duration    `toml:"metric_interval"`
	ServiceName        *string           `toml:"service_name"`
	SlowDBWriteMS      *int              `toml:"slow_db_write_ms"`
	ForwardToProviders *bool             `toml:"forward_to_providers"`
}

func (o observabilityOverlay) Apply(dst *ObservabilityConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.RetentionDays != nil {
		dst.RetentionDays = *o.RetentionDays
	}
	if o.MaxGlobalBytes != nil {
		dst.MaxGlobalBytes = *o.MaxGlobalBytes
	}
	if o.AgentProbeTimeout != nil {
		dst.AgentProbeTimeout = *o.AgentProbeTimeout
	}
	o.Transcripts.Apply(&dst.Transcripts)
	if o.OTel != nil {
		o.OTel.Apply(&dst.OTel)
	}
}

func (o observabilityTranscriptsOverlay) Apply(dst *ObservabilityTranscriptConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.SegmentBytes != nil {
		dst.SegmentBytes = *o.SegmentBytes
	}
	if o.MaxBytesPerSession != nil {
		dst.MaxBytesPerSession = *o.MaxBytesPerSession
	}
}

// Apply writes file values over the OTEL_* layer: a written field drops its environment source, and
// a non-empty headers table replaces environment headers entirely. An empty endpoint means unset, so
// OTEL_EXPORTER_OTLP_ENDPOINT still applies.
func (o observabilityOTelOverlay) Apply(dst *ObservabilityOTelConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.Protocol != nil {
		dst.Protocol, dst.env.protocol = *o.Protocol, ""
	}
	if o.Endpoint != nil && *o.Endpoint != "" {
		dst.Endpoint, dst.env.endpoint = *o.Endpoint, ""
	}
	if len(o.Headers) > 0 {
		dst.Headers = maps.Clone(o.Headers)
		dst.env.headers, dst.env.headersInvalid = nil, false
	}
	if o.Traces != nil {
		dst.Traces = *o.Traces
	}
	if o.Metrics != nil {
		dst.Metrics = *o.Metrics
	}
	if o.SampleRatio != nil {
		dst.SampleRatio = *o.SampleRatio
	}
	if o.MetricInterval != nil {
		dst.MetricInterval = *o.MetricInterval
	}
	if o.ServiceName != nil {
		dst.ServiceName, dst.env.serviceName = *o.ServiceName, ""
	}
	if o.SlowDBWriteMS != nil {
		dst.SlowDBWriteMS = *o.SlowDBWriteMS
	}
	if o.ForwardToProviders != nil {
		dst.ForwardToProviders = *o.ForwardToProviders
	}
}
