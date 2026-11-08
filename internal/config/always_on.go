package config

import "time"

type AlwaysOnConfig struct {
	AutoAcceptPerDay      int           `toml:"auto_accept_per_day"`
	CheckInTools          string        `toml:"check_in_tools"`
	CheckInMaxToolCalls   int           `toml:"check_in_max_tool_calls"`
	CheckInMaxWriteCalls  int           `toml:"check_in_max_write_calls"`
	CheckInAuditRetention time.Duration `toml:"check_in_audit_retention"`

	UnattendedTurnLimit int64         `toml:"unattended_turn_limit"`
	RotationThreshold   float64       `toml:"rotation_threshold"`
	ApprovalTimeout     time.Duration `toml:"approval_timeout"`
	MaxProposals        int           `toml:"max_proposals"`
	TriageTimeout       time.Duration `toml:"triage_timeout"`
	MaxActiveInstances  int           `toml:"max_active_instances"`
}

type alwaysOnOverlay struct {
	AutoAcceptPerDay      *int           `toml:"auto_accept_per_day"`
	CheckInTools          *string        `toml:"check_in_tools"`
	CheckInMaxToolCalls   *int           `toml:"check_in_max_tool_calls"`
	CheckInMaxWriteCalls  *int           `toml:"check_in_max_write_calls"`
	CheckInAuditRetention *time.Duration `toml:"check_in_audit_retention"`

	UnattendedTurnLimit *int64         `toml:"unattended_turn_limit"`
	RotationThreshold   *float64       `toml:"rotation_threshold"`
	ApprovalTimeout     *time.Duration `toml:"approval_timeout"`
	MaxProposals        *int           `toml:"max_proposals"`
	TriageTimeout       *time.Duration `toml:"triage_timeout"`
	MaxActiveInstances  *int           `toml:"max_active_instances"`
}

func (o alwaysOnOverlay) Apply(dst *AlwaysOnConfig) {
	if o.AutoAcceptPerDay != nil {
		dst.AutoAcceptPerDay = *o.AutoAcceptPerDay
	}
	if o.CheckInTools != nil {
		dst.CheckInTools = *o.CheckInTools
	}
	if o.CheckInMaxToolCalls != nil {
		dst.CheckInMaxToolCalls = *o.CheckInMaxToolCalls
	}
	if o.CheckInMaxWriteCalls != nil {
		dst.CheckInMaxWriteCalls = *o.CheckInMaxWriteCalls
	}
	if o.CheckInAuditRetention != nil {
		dst.CheckInAuditRetention = *o.CheckInAuditRetention
	}

	if o.UnattendedTurnLimit != nil {
		dst.UnattendedTurnLimit = *o.UnattendedTurnLimit
	}
	if o.RotationThreshold != nil {
		dst.RotationThreshold = *o.RotationThreshold
	}
	if o.ApprovalTimeout != nil {
		dst.ApprovalTimeout = *o.ApprovalTimeout
	}
	if o.MaxProposals != nil {
		dst.MaxProposals = *o.MaxProposals
	}
	if o.TriageTimeout != nil {
		dst.TriageTimeout = *o.TriageTimeout
	}
	if o.MaxActiveInstances != nil {
		dst.MaxActiveInstances = *o.MaxActiveInstances
	}
}

func DefaultAlwaysOnConfig() AlwaysOnConfig {
	return AlwaysOnConfig{
		AutoAcceptPerDay:      20,
		CheckInTools:          "enabled",
		CheckInMaxToolCalls:   25,
		CheckInMaxWriteCalls:  5,
		CheckInAuditRetention: 720 * time.Hour,
		UnattendedTurnLimit:   300,
		RotationThreshold:     0.8,
		MaxActiveInstances:    10,
		ApprovalTimeout:       12 * time.Hour,
		MaxProposals:          2,
		TriageTimeout:         10 * time.Minute,
	}
}
