package webhook

import (
	"time"

	"github.com/compozy/compozy/engine/schema"
)

// Config represents per-workflow webhook settings and routing events.
type Config struct {
	Slug   string        `json:"slug"             yaml:"slug"             mapstructure:"slug"`
	Method string        `json:"method,omitempty" yaml:"method,omitempty" mapstructure:"method"`
	Events []EventConfig `json:"events"           yaml:"events"           mapstructure:"events"`
	Verify *VerifySpec   `json:"verify,omitempty" yaml:"verify,omitempty" mapstructure:"verify"`
	Dedupe *DedupeSpec   `json:"dedupe,omitempty" yaml:"dedupe,omitempty" mapstructure:"dedupe"`
}

// EventConfig defines a single routable event within a webhook trigger.
type EventConfig struct {
	Name   string            `json:"name"             yaml:"name"             mapstructure:"name"`
	Filter string            `json:"filter"           yaml:"filter"           mapstructure:"filter"`
	Input  map[string]string `json:"input"            yaml:"input"            mapstructure:"input"`
	Schema *schema.Schema    `json:"schema,omitempty" yaml:"schema,omitempty" mapstructure:"schema"`
}

// VerifySpec defines signature verification options for webhook requests.
type VerifySpec struct {
	Strategy string        `json:"strategy,omitempty" yaml:"strategy,omitempty" mapstructure:"strategy"`
	Secret   string        `json:"secret,omitempty"   yaml:"secret,omitempty"   mapstructure:"strategy"`
	Header   string        `json:"header,omitempty"   yaml:"header,omitempty"   mapstructure:"header"`
	Skew     time.Duration `json:"skew,omitempty"     yaml:"skew,omitempty"     mapstructure:"skey"`
}

// DedupeSpec controls idempotency behavior for webhook requests.
type DedupeSpec struct {
	Enabled bool   `json:"enabled"       yaml:"enabled"       mapstructure:"enabled"`
	TTL     string `json:"ttl,omitempty" yaml:"ttl,omitempty" mapstructure:"ttl"`
	Key     string `json:"key,omitempty" yaml:"key,omitempty" mapstructure:"key"`
}

// ApplyDefaults sets default values for optional fields.
func ApplyDefaults(cfg *Config) {
	if cfg.Method == "" {
		cfg.Method = "POST"
	}
	if cfg.Verify != nil && cfg.Verify.Strategy == "" {
		cfg.Verify.Strategy = StrategyNone
	}
}

// ToVerifyConfig converts VerifySpec to runtime VerifyConfig used by verifiers.
func (v *VerifySpec) ToVerifyConfig() VerifyConfig {
	if v == nil {
		return VerifyConfig{Strategy: StrategyNone}
	}
	return VerifyConfig{Strategy: v.Strategy, Secret: v.Secret, Header: v.Header, Skew: v.Skew}
}
