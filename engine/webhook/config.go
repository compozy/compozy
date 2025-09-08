package webhook

import (
	"github.com/compozy/compozy/engine/schema"
)

// Config defines per-workflow webhook settings and routing events.
type Config struct {
	Slug   string        `json:"slug"             yaml:"slug"`
	Method string        `json:"method,omitempty" yaml:"method,omitempty"`
	Verify *VerifySpec   `json:"verify,omitempty" yaml:"verify,omitempty"`
	Dedupe *DedupeSpec   `json:"dedupe,omitempty" yaml:"dedupe,omitempty"`
	Events []EventConfig `json:"events"           yaml:"events"`
}

// EventConfig defines a single routable event within a webhook trigger.
type EventConfig struct {
	Name   string            `json:"name"             yaml:"name"`
	Filter string            `json:"filter"           yaml:"filter"`
	Input  map[string]string `json:"input"            yaml:"input"`
	Schema *schema.Schema    `json:"schema,omitempty" yaml:"schema,omitempty"`
}

// VerifySpec defines signature verification options for webhook requests.
// strategy: none|hmac|stripe|github
type VerifySpec struct {
	Strategy string `json:"strategy"         yaml:"strategy"`
	Secret   string `json:"secret,omitempty" yaml:"secret,omitempty"`
	Header   string `json:"header,omitempty" yaml:"header,omitempty"`
}

// DedupeSpec controls idempotency behavior for webhook requests.
type DedupeSpec struct {
	Enabled bool   `json:"enabled"       yaml:"enabled"`
	TTL     string `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Key     string `json:"key,omitempty" yaml:"key,omitempty"`
}

// ApplyDefaults applies webhook-default values.
func ApplyDefaults(cfg *Config) {
	if cfg == nil {
		return
	}
	if cfg.Method == "" {
		cfg.Method = "POST"
	}
	if cfg.Dedupe != nil && cfg.Dedupe.Enabled && cfg.Dedupe.TTL == "" {
		cfg.Dedupe.TTL = "5m"
	}
}
