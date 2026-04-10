package model

import "time"

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
	Name                       string
	Round                      int
	Provider                   string
	PR                         string
	Nitpicks                   bool
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
	Mode                       ExecutionMode
	OutputFormat               OutputFormat
	Verbose                    bool
	TUI                        bool
	Persist                    bool
	EnableExecutableExtensions bool
	RunID                      string
	ParentRunID                string
	PromptText                 string
	PromptFile                 string
	ReadPromptStdin            bool
	ResolvedPromptText         string
	IncludeCompleted           bool
	IncludeResolved            bool
	Timeout                    time.Duration
	MaxRetries                 int
	RetryBackoffMultiplier     float64
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
		cfg.ReasoningEffort = "medium"
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
}
