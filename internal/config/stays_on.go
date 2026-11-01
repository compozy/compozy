package config

import "time"

type StaysOnConfig struct {
	ApprovalTimeout    time.Duration `toml:"approval_timeout"`
	MaxActiveInstances int           `toml:"max_active_instances"`
}

type staysOnOverlay struct {
	ApprovalTimeout    *time.Duration `toml:"approval_timeout"`
	MaxActiveInstances *int           `toml:"max_active_instances"`
}

func (o staysOnOverlay) Apply(dst *StaysOnConfig) {
	if o.ApprovalTimeout != nil {
		dst.ApprovalTimeout = *o.ApprovalTimeout
	}
	if o.MaxActiveInstances != nil {
		dst.MaxActiveInstances = *o.MaxActiveInstances
	}
}
