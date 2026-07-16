package model

import (
	"strings"
	"time"
)

const DefaultReasoningEffort = "medium"

// ExplicitRuntimeFlags tracks which runtime fields were explicitly overridden
// by the current caller, using CLI-compatible `Flags().Changed(...)` semantics.
type ExplicitRuntimeFlags struct {
	IDE             bool
	Model           bool
	ReasoningEffort bool
	AccessMode      bool
}

type RuntimeConfig struct {
	WorkspaceRoot              string
	RunsDir                    string `json:"-"`
	Name                       string
	WorkflowName               string `json:"-"`
	Round                      int
	Provider                   string
	PR                         string
	Nitpicks                   bool
	ExecutionScope             *ExecutionScope `json:"-"`
	ReviewsDir                 string
	TasksDir                   string
	DryRun                     bool
	AutoCommit                 bool
	Concurrent                 int
	BatchSize                  int
	IDE                        string
	Model                      string
	AddDirs                    []string
	TailLines                  int
	ReasoningEffort            string
	AccessMode                 string
	AgentName                  string
	ExplicitRuntime            ExplicitRuntimeFlags
	TaskRuntimeRules           []TaskRuntimeRule
	Mode                       ExecutionMode
	OutputFormat               OutputFormat
	Verbose                    bool
	TUI                        bool
	Persist                    bool
	EnableExecutableExtensions bool
	DaemonOwned                bool
	RunID                      string
	ParentRunID                string
	PromptText                 string
	SystemPrompt               string
	TargetTaskNumber           *int
	PromptFile                 string
	ReadPromptStdin            bool
	ResolvedPromptText         string
	IncludeCompleted           bool
	Recursive                  bool
	RecoveryAttempt            int
	IncludeResolved            bool
	Timeout                    time.Duration
	MaxRetries                 int
	RetryBackoffMultiplier     float64
	SoundEnabled               bool
	SoundOnCompleted           string
	SoundOnFailed              string
	SoundOnParked              string
	// Resolved stall-detection knobs. StallEnabled and StallRetries are pointers
	// so ApplyDefaults can distinguish an explicit override (false / 0) from an
	// unset value that must fall back to the on-by-default policy.
	StallEnabled           *bool
	StallTimeout           time.Duration
	ChildStallTimeout      time.Duration
	TerminalCommandTimeout time.Duration
	StallRetries           *int
	JobControls            *JobControlRegistry `json:"-"`
}

// StallPolicy is the resolved stall-detection configuration for a run,
// populated from RuntimeConfig by ApplyDefaults. Nested budgets:
// IdleTimeout < ChildTimeout, and TerminalCap is a generous backstop only.
type StallPolicy struct {
	Enabled      bool          // on by default; disable flag
	IdleTimeout  time.Duration // per-attempt idle window; "any update resets"
	ChildTimeout time.Duration // daemon backstop; strictly > IdleTimeout
	TerminalCap  time.Duration // absolute per-command wall-clock backstop
	Retries      int           // clean-state stall retries before park (default 1)
}

// StallPolicy returns the effective, resolved stall-detection policy. Callers
// should invoke ApplyDefaults first so the durations and defaults are populated;
// the accessor still resolves Enabled and Retries defensively when they are unset.
func (cfg *RuntimeConfig) StallPolicy() StallPolicy {
	enabled := cfg.StallEnabled == nil || *cfg.StallEnabled
	retries := DefaultStallRetries
	if cfg.StallRetries != nil {
		retries = *cfg.StallRetries
	}
	return StallPolicy{
		Enabled:      enabled,
		IdleTimeout:  cfg.StallTimeout,
		ChildTimeout: cfg.ChildStallTimeout,
		TerminalCap:  cfg.TerminalCommandTimeout,
		Retries:      retries,
	}
}

func (cfg *RuntimeConfig) ApplyDefaults() {
	if cfg.Concurrent <= 0 {
		cfg.Concurrent = 1
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1
	}
	if cfg.IDE == "" {
		cfg.IDE = IDECodex
	}
	if cfg.TailLines < 0 {
		cfg.TailLines = 0
	}
	if cfg.ReasoningEffort == "" {
		cfg.ReasoningEffort = DefaultReasoningEffort
	}
	if cfg.AccessMode == "" {
		cfg.AccessMode = AccessModeFull
	}
	if cfg.Mode == "" {
		cfg.Mode = ExecutionModePRReview
	}
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = OutputFormatText
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultActivityTimeout
	}
	if cfg.RetryBackoffMultiplier <= 0 {
		cfg.RetryBackoffMultiplier = 1.5
	}
	if cfg.SoundEnabled {
		cfg.SoundOnCompleted = strings.TrimSpace(cfg.SoundOnCompleted)
		if cfg.SoundOnCompleted == "" {
			cfg.SoundOnCompleted = DefaultSoundOnCompleted
		}
		cfg.SoundOnFailed = strings.TrimSpace(cfg.SoundOnFailed)
		if cfg.SoundOnFailed == "" {
			cfg.SoundOnFailed = DefaultSoundOnFailed
		}
	}
	cfg.applyStallDefaults()
}

// applyStallDefaults resolves the stall-detection policy to safe, on-by-default
// values and enforces the child-timeout > idle-timeout invariant. The parked
// alert sound defaults unconditionally because the parked alert is part of the
// on-by-default stall policy, independent of the general sound feature.
func (cfg *RuntimeConfig) applyStallDefaults() {
	if cfg.StallEnabled == nil {
		enabled := true
		cfg.StallEnabled = &enabled
	}
	if cfg.StallTimeout <= 0 {
		cfg.StallTimeout = DefaultStallIdleTimeout
	}
	if cfg.ChildStallTimeout <= 0 {
		cfg.ChildStallTimeout = DefaultStallChildTimeout
	}
	// The daemon backstop budget must stay strictly larger than the fast idle
	// window so the in-attempt watchdog heals first. Correct any override that
	// violates the invariant rather than silently trusting it.
	if cfg.ChildStallTimeout <= cfg.StallTimeout {
		cfg.ChildStallTimeout = cfg.StallTimeout * 2
	}
	if cfg.TerminalCommandTimeout <= 0 {
		cfg.TerminalCommandTimeout = DefaultStallTerminalCap
	}
	if cfg.StallRetries == nil {
		retries := DefaultStallRetries
		cfg.StallRetries = &retries
	}
	cfg.SoundOnParked = strings.TrimSpace(cfg.SoundOnParked)
	if cfg.SoundOnParked == "" {
		cfg.SoundOnParked = DefaultSoundOnParked
	}
}

// DefaultSoundOnCompleted is the preset played on run.completed when sound is
// enabled and the user has not set an explicit preset or path.
const DefaultSoundOnCompleted = "glass"

// DefaultSoundOnFailed is the preset played on run.failed / run.cancelled when
// sound is enabled and the user has not set an explicit preset or path.
const DefaultSoundOnFailed = "basso"

// DefaultSoundOnParked is the preset played when a job is parked after its
// clean-state stall retry. Defaulted unconditionally as part of the
// on-by-default stall policy.
const DefaultSoundOnParked = "funk"
