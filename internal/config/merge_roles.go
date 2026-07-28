package config

import "time"

type rolesOverlay struct {
	Coordinator       coordinatorRoleOverlay      `toml:"coordinator"`
	Dream             roleOverlay                 `toml:"dream"`
	CheckpointSummary roleOverlay                 `toml:"checkpoint_summary"`
	MemoryExtractor   roleOverlay                 `toml:"memory_extractor"`
	AutoTitle         roleOverlay                 `toml:"auto_title"`
	MemoryController  memoryControllerRoleOverlay `toml:"memory_controller"`
}

type roleOverlay struct {
	Enabled         *bool           `toml:"enabled"`
	Agent           *string         `toml:"agent"`
	Provider        *string         `toml:"provider"`
	Model           *string         `toml:"model"`
	ReasoningEffort *string         `toml:"reasoning_effort"`
	FallbackChain   *[]RoleFallback `toml:"fallback_chain"`
}

type coordinatorRoleOverlay struct {
	roleOverlay
	TTL                           *time.Duration `toml:"ttl"`
	MaxChildren                   *int           `toml:"max_children"`
	MaxActiveSessionsPerWorkspace *int           `toml:"max_active_sessions_per_workspace"`
}

type memoryControllerRoleOverlay struct {
	Enabled         *bool           `toml:"enabled"`
	Provider        *string         `toml:"provider"`
	Model           *string         `toml:"model"`
	ReasoningEffort *string         `toml:"reasoning_effort"`
	Timeout         *time.Duration  `toml:"timeout"`
	TopK            *int            `toml:"top_k"`
	PromptVersion   *string         `toml:"prompt_version"`
	MaxTokensOut    *int            `toml:"max_tokens_out"`
	FallbackChain   *[]RoleFallback `toml:"fallback_chain"`
}

func (o rolesOverlay) Apply(dst *RolesConfig) {
	o.Coordinator.Apply(&dst.Coordinator)
	o.Dream.Apply(&dst.Dream)
	o.CheckpointSummary.Apply(&dst.CheckpointSummary)
	o.MemoryExtractor.Apply(&dst.MemoryExtractor)
	o.AutoTitle.Apply(&dst.AutoTitle)
	o.MemoryController.Apply(&dst.MemoryController)
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
	if o.FallbackChain != nil {
		dst.FallbackChain = append([]RoleFallback(nil), (*o.FallbackChain)...)
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

func (o memoryControllerRoleOverlay) Apply(dst *MemoryControllerRoleConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
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
	if o.Timeout != nil {
		dst.Timeout = *o.Timeout
	}
	if o.TopK != nil {
		dst.TopK = *o.TopK
	}
	if o.PromptVersion != nil {
		dst.PromptVersion = *o.PromptVersion
	}
	if o.MaxTokensOut != nil {
		dst.MaxTokensOut = *o.MaxTokensOut
	}
	if o.FallbackChain != nil {
		dst.FallbackChain = append([]RoleFallback(nil), (*o.FallbackChain)...)
	}
}
