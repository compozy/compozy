package config

import "time"

type StaysOnConfig struct {
	RotationThreshold  float64       `toml:"rotation_threshold"`
	ApprovalTimeout    time.Duration `toml:"approval_timeout"`
	MaxProposals       int           `toml:"max_proposals"`
	TriageTimeout      time.Duration `toml:"triage_timeout"`
	MaxActiveInstances int           `toml:"max_active_instances"`
}

type staysOnOverlay struct {
	RotationThreshold  *float64       `toml:"rotation_threshold"`
	ApprovalTimeout    *time.Duration `toml:"approval_timeout"`
	MaxProposals       *int           `toml:"max_proposals"`
	TriageTimeout      *time.Duration `toml:"triage_timeout"`
	MaxActiveInstances *int           `toml:"max_active_instances"`
}

func (o staysOnOverlay) Apply(dst *StaysOnConfig) {
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
