package webhook

import (
	"time"

	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/config"
)

// Config represents per-workflow webhook settings and routing events.
type Config struct {
	Slug   string        `json:"slug"             yaml:"slug"`
	Method string        `json:"method,omitempty" yaml:"method,omitempty"`
	Events []EventConfig `json:"events"           yaml:"events"`
	Verify *VerifySpec   `json:"verify,omitempty" yaml:"verify,omitempty"`
	Dedupe *DedupeSpec   `json:"dedupe,omitempty" yaml:"dedupe,omitempty"`
}

// EventConfig defines a single routable event within a webhook trigger.
type EventConfig struct {
	Name   string            `json:"name"             yaml:"name"`
	Filter string            `json:"filter"           yaml:"filter"`
	Input  map[string]string `json:"input"            yaml:"input"`
	Schema *schema.Schema    `json:"schema,omitempty" yaml:"schema,omitempty"`
}

// VerifySpec defines signature verification options for webhook requests.
type VerifySpec struct {
	Strategy string        `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	Secret   string        `json:"secret,omitempty"   yaml:"secret,omitempty"`
	Header   string        `json:"header,omitempty"   yaml:"header,omitempty"`
	Skew     time.Duration `json:"skew,omitempty"     yaml:"skew,omitempty"`
}

// DedupeSpec controls idempotency behavior for webhook requests.
type DedupeSpec struct {
	Enabled bool   `json:"enabled"       yaml:"enabled"`
	TTL     string `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Key     string `json:"key,omitempty" yaml:"key,omitempty"`
}

// ApplyDefaults sets default values for optional fields.
func ApplyDefaults(cfg *Config) {
	globalCfg := config.Get()
	if cfg.Method == "" {
		cfg.Method = globalCfg.Webhooks.DefaultMethod
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
