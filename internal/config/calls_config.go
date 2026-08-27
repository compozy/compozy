package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/contracts"
)

const (
	defaultCallsDedupWindow = "30s"
	callsMaxBatchPath       = "calls.max_batch"
)

// CallsConfig controls call admission, lifecycle, result, and mailbox bounds.
type CallsConfig struct {
	MaxDepth         int                 `toml:"max_depth"`
	MaxBatch         int                 `toml:"max_batch"`
	MaxChildren      int                 `toml:"max_children"`
	MaxActivePerRoot int                 `toml:"max_active_per_root"`
	IdleTTL          string              `toml:"idle_ttl"`
	OperationTimeout string              `toml:"operation_timeout"`
	Results          CallsResultsConfig  `toml:"results"`
	Messages         CallsMessagesConfig `toml:"messages"`
}

// CallsResultsConfig controls result budgets and overflow behavior.
type CallsResultsConfig struct {
	DefaultBudget string `toml:"default_budget"`
	MaxBudget     string `toml:"max_budget"`
	Overflow      string `toml:"overflow"`
}

// CallsMessagesConfig controls mailbox rate, deduplication, and payload bounds.
type CallsMessagesConfig struct {
	RateLimitPerMinute int    `toml:"rate_limit_per_minute"`
	DedupWindow        string `toml:"dedup_window"`
	PendingCap         int    `toml:"pending_cap"`
	MaxBytes           string `toml:"max_bytes"`
}

// DefaultCallsConfig returns the exact built-in call bounds.
func DefaultCallsConfig() CallsConfig {
	resultPolicy := contracts.DefaultCallsResultsConfig()
	return CallsConfig{
		MaxDepth:         3,
		MaxBatch:         8,
		MaxChildren:      5,
		MaxActivePerRoot: 32,
		IdleTTL:          "1h",
		OperationTimeout: "30s",
		Results: CallsResultsConfig{
			DefaultBudget: fmt.Sprintf("%dKiB", resultPolicy.DefaultBudget.MaxBytes>>10),
			MaxBudget:     fmt.Sprintf("%dMiB", resultPolicy.MaxBudget>>20),
			Overflow:      string(contracts.OverflowStore),
		},
		Messages: CallsMessagesConfig{
			RateLimitPerMinute: 30,
			DedupWindow:        defaultCallsDedupWindow,
			PendingCap:         50,
			MaxBytes:           "64KiB",
		},
	}
}

// Validate ensures every configured call bound is positive and internally consistent.
func (c CallsConfig) Validate() error {
	caps := []struct {
		path  string
		value int
	}{
		{path: "calls.max_depth", value: c.MaxDepth},
		{path: callsMaxBatchPath, value: c.MaxBatch},
		{path: "calls.max_children", value: c.MaxChildren},
		{path: "calls.max_active_per_root", value: c.MaxActivePerRoot},
	}
	for _, bound := range caps {
		if bound.value <= 0 {
			return fmt.Errorf("%s must be positive: %d", bound.path, bound.value)
		}
	}
	if _, err := parsePositiveDuration(c.IdleTTL); err != nil {
		return fmt.Errorf("calls.idle_ttl: %w", err)
	}
	if _, err := parsePositiveDuration(c.OperationTimeout); err != nil {
		return fmt.Errorf("calls.operation_timeout: %w", err)
	}
	if err := c.Results.Validate(); err != nil {
		return err
	}
	return c.Messages.Validate()
}

// Validate ensures result budgets and overflow policy are usable.
func (c CallsResultsConfig) Validate() error {
	_, err := c.normalizedPolicy()
	return err
}

func (c CallsResultsConfig) normalizedPolicy() (contracts.CallsResultsConfig, error) {
	defaultBytes, err := ParseByteSize(c.DefaultBudget)
	if err != nil {
		return contracts.CallsResultsConfig{}, fmt.Errorf("calls.results.default_budget: %w", err)
	}
	maxBytes, err := ParseByteSize(c.MaxBudget)
	if err != nil {
		return contracts.CallsResultsConfig{}, fmt.Errorf("calls.results.max_budget: %w", err)
	}
	if defaultBytes > maxBytes {
		return contracts.CallsResultsConfig{}, fmt.Errorf(
			"calls.results: default_budget %q exceeds max_budget %q",
			c.DefaultBudget,
			c.MaxBudget,
		)
	}
	mode := contracts.OverflowMode(strings.TrimSpace(c.Overflow))
	if mode != contracts.OverflowStore && mode != contracts.OverflowReject {
		return contracts.CallsResultsConfig{}, fmt.Errorf(
			`calls.results.overflow must be "store" or "reject": %q`,
			c.Overflow,
		)
	}
	return contracts.CallsResultsConfig{
		DefaultBudget: contracts.ByteBudget{MaxBytes: defaultBytes, Overflow: mode},
		MaxBudget:     maxBytes,
	}, nil
}

// ContractPolicy converts the configured strings into the shared parsed budget contract.
func (c CallsResultsConfig) ContractPolicy() (contracts.CallsResultsConfig, error) {
	return c.normalizedPolicy()
}

// Validate ensures mailbox bounds are positive and parseable.
func (c CallsMessagesConfig) Validate() error {
	if c.RateLimitPerMinute <= 0 {
		return fmt.Errorf(
			"calls.messages.rate_limit_per_minute must be positive: %d",
			c.RateLimitPerMinute,
		)
	}
	if _, err := parsePositiveDuration(c.DedupWindow); err != nil {
		return fmt.Errorf("calls.messages.dedup_window: %w", err)
	}
	if c.PendingCap <= 0 {
		return fmt.Errorf("calls.messages.pending_cap must be positive: %d", c.PendingCap)
	}
	if _, err := ParseByteSize(c.MaxBytes); err != nil {
		return fmt.Errorf("calls.messages.max_bytes: %w", err)
	}
	return nil
}

// IdleTTLDuration returns the validated parked-session idle ceiling.
func (c CallsConfig) IdleTTLDuration() (time.Duration, error) {
	duration, err := parsePositiveDuration(c.IdleTTL)
	if err != nil {
		return 0, fmt.Errorf("calls.idle_ttl: %w", err)
	}
	return duration, nil
}

// OperationTimeoutDuration returns the bound for detached call mutations and completion work.
func (c CallsConfig) OperationTimeoutDuration() (time.Duration, error) {
	duration, err := parsePositiveDuration(c.OperationTimeout)
	if err != nil {
		return 0, fmt.Errorf("calls.operation_timeout: %w", err)
	}
	return duration, nil
}

// DedupWindowDuration returns the validated mailbox deduplication window.
func (c CallsMessagesConfig) DedupWindowDuration() (time.Duration, error) {
	duration, err := parsePositiveDuration(c.DedupWindow)
	if err != nil {
		return 0, fmt.Errorf("calls.messages.dedup_window: %w", err)
	}
	return duration, nil
}

// ParseByteSize parses a positive binary byte size such as "256KiB" or "4MiB".
func ParseByteSize(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	units := []struct {
		suffix     string
		multiplier int
	}{
		{suffix: "GiB", multiplier: 1 << 30},
		{suffix: "MiB", multiplier: 1 << 20},
		{suffix: "KiB", multiplier: 1 << 10},
		{suffix: "B", multiplier: 1},
	}
	for _, unit := range units {
		if !strings.HasSuffix(value, unit.suffix) {
			continue
		}
		number := strings.TrimSpace(strings.TrimSuffix(value, unit.suffix))
		parsed, err := strconv.Atoi(number)
		if err != nil || parsed <= 0 {
			return 0, fmt.Errorf(`must be a positive byte size such as "256KiB": %q`, raw)
		}
		if parsed > int(^uint(0)>>1)/unit.multiplier {
			return 0, fmt.Errorf("byte size exceeds platform limit: %q", raw)
		}
		return parsed * unit.multiplier, nil
	}
	return 0, fmt.Errorf(`must be a positive byte size such as "256KiB": %q`, raw)
}

func parsePositiveDuration(raw string) (time.Duration, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("must be a positive duration: %q", raw)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("must be a positive duration: %q", raw)
	}
	return duration, nil
}
