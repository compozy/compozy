package config

import (
	"time"

	speedpkg "github.com/compozy/compozy/internal/speed"
)

type rolesOverlay struct {
	Coordinator coordinatorRoleOverlay `toml:"coordinator"`
	AutoTitle   roleOverlay            `toml:"auto_title"`
}

type roleOverlay struct {
	Enabled         *bool                 `toml:"enabled"`
	Agent           *string               `toml:"agent"`
	Provider        *string               `toml:"provider"`
	Model           *string               `toml:"model"`
	ReasoningEffort *string               `toml:"reasoning_effort"`
	Speed           *string               `toml:"speed"`
	ACPOptions      *[]ACPOptionSelection `toml:"acp_options"`
	FallbackChain   *[]RoleFallback       `toml:"fallback_chain"`
}

type coordinatorRoleOverlay struct {
	roleOverlay
	TTL                           *time.Duration `toml:"ttl"`
	MaxChildren                   *int           `toml:"max_children"`
	MaxActiveSessionsPerWorkspace *int           `toml:"max_active_sessions_per_workspace"`
}

func (o rolesOverlay) Apply(dst *RolesConfig) {
	o.Coordinator.Apply(&dst.Coordinator)
	o.AutoTitle.Apply(&dst.AutoTitle)
}

func (o roleOverlay) Apply(dst *RoleConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.Agent != nil {
		dst.Agent = *o.Agent
	}
	if o.Provider != nil {
		dst.Provider = *o.Provider
	}
	if o.Model != nil {
		dst.Model = *o.Model
	}
	if o.ReasoningEffort != nil {
		dst.ReasoningEffort = *o.ReasoningEffort
	}
	if o.Speed != nil {
		dst.Speed = speedpkg.Speed(*o.Speed)
	}
	if o.ACPOptions != nil {
		dst.ACPOptions = CloneACPOptionSelections(*o.ACPOptions)
	}
	if o.FallbackChain != nil {
		dst.FallbackChain = cloneRoleFallbacks(*o.FallbackChain)
	}
}

func (o coordinatorRoleOverlay) Apply(dst *CoordinatorRoleConfig) {
	o.roleOverlay.Apply(&dst.RoleConfig)
	if o.TTL != nil {
		dst.TTL = *o.TTL
	}
	if o.MaxChildren != nil {
		dst.MaxChildren = *o.MaxChildren
	}
	if o.MaxActiveSessionsPerWorkspace != nil {
		dst.MaxActiveSessionsPerWorkspace = *o.MaxActiveSessionsPerWorkspace
	}
}
