package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	loopRetryMaxAttempts     = 10
	loopPositiveValueMessage = "must be > 0"
)

// LoopRetryDefaultConfig controls retry defaults for mechanical node families.
type LoopRetryDefaultConfig struct {
	MaxAttempts int    `toml:"max_attempts"`
	BackoffBase string `toml:"backoff_base"`
	BackoffMax  string `toml:"backoff_max"`
}

// LoopLivenessDefaultConfig controls evidence-based silence evaluation.
type LoopLivenessDefaultConfig struct {
	SilenceWindow string `toml:"silence_window"`
}

// LoopResumeDefaultConfig controls bounded dead-session resume.
type LoopResumeDefaultConfig struct {
	DeathStreakLimit int `toml:"death_streak_limit"`
}

// LoopPredicateDefaultConfig controls CEL evaluation cost.
type LoopPredicateDefaultConfig struct {
	CostLimit uint64 `toml:"cost_limit"`
}

// LoopWaitDefaultConfig controls wait admission retries.
type LoopWaitDefaultConfig struct {
	AdmissionAttempts      int    `toml:"admission_attempts"`
	AdmissionRetryInterval string `toml:"admission_retry_interval"`
}

// LoopAdmissionDefaultConfig controls watch admission tombstone retention.
type LoopAdmissionDefaultConfig struct {
	TombstoneHorizon string `toml:"tombstone_horizon"`
}

// LoopAutopauseRule is one ordered first-match-wins pause rule.
type LoopAutopauseRule struct {
	Match  string `toml:"match"`
	Action string `toml:"action"`
}

// LoopBreakerConfig is the daemon-global target breaker policy.
type LoopBreakerConfig struct {
	Threshold     int    `toml:"threshold"`
	ProbeInterval string `toml:"probe_interval"`
}

func (c LoopRetryDefaultConfig) validate(path string) error {
	if c.MaxAttempts < 0 || c.MaxAttempts > loopRetryMaxAttempts {
		return ValidationError{Path: path + ".max_attempts", Message: "must be between 0 and 10"}
	}
	base, err := parsePositiveLoopDuration(path+".backoff_base", c.BackoffBase, false)
	if err != nil {
		return err
	}
	maximum, err := parsePositiveLoopDuration(path+".backoff_max", c.BackoffMax, false)
	if err != nil {
		return err
	}
	if base > maximum {
		return ValidationError{
			Path:    path + ".backoff_base",
			Message: fmt.Sprintf("must be <= %s", path+".backoff_max"),
		}
	}
	return nil
}

func (c LoopLivenessDefaultConfig) validate(path string) error {
	_, err := parsePositiveLoopDuration(path+".silence_window", c.SilenceWindow, true)
	return err
}

func (c LoopResumeDefaultConfig) validate(path string) error {
	if c.DeathStreakLimit <= 0 {
		return ValidationError{Path: path + ".death_streak_limit", Message: loopPositiveValueMessage}
	}
	return nil
}

func (c LoopPredicateDefaultConfig) validate(path string) error {
	if c.CostLimit == 0 {
		return ValidationError{Path: path + ".cost_limit", Message: loopPositiveValueMessage}
	}
	return nil
}

func (c LoopWaitDefaultConfig) validate(path string) error {
	if c.AdmissionAttempts <= 0 {
		return ValidationError{Path: path + ".admission_attempts", Message: loopPositiveValueMessage}
	}
	_, err := parsePositiveLoopDuration(path+".admission_retry_interval", c.AdmissionRetryInterval, false)
	return err
}

func (c LoopAdmissionDefaultConfig) validate(path string) error {
	_, err := parsePositiveLoopDuration(path+".tombstone_horizon", c.TombstoneHorizon, false)
	return err
}

func (c LoopBreakerConfig) validate(path string) error {
	if c.Threshold <= 0 {
		return ValidationError{Path: path + ".threshold", Message: loopPositiveValueMessage}
	}
	_, err := parsePositiveLoopDuration(path+".probe_interval", c.ProbeInterval, false)
	return err
}

func parsePositiveLoopDuration(path, raw string, allowZero bool) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, ValidationError{Path: path, Message: fmt.Sprintf("must be a duration: %q", raw)}
	}
	if duration < 0 || (!allowZero && duration == 0) {
		operator := "> 0"
		if allowZero {
			operator = ">= 0"
		}
		return 0, ValidationError{Path: path, Message: "must be " + operator}
	}
	return duration, nil
}
