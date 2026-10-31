package contract

// ProviderRuntimeStrategy identifies how a provider applies runtime controls.
type ProviderRuntimeStrategy string

const (
	ProviderRuntimeStrategySessionConfig   ProviderRuntimeStrategy = "session_config"
	ProviderRuntimeStrategyLaunchArg       ProviderRuntimeStrategy = "launch_arg"
	ProviderRuntimeStrategyProviderManaged ProviderRuntimeStrategy = "provider_managed"
)

// ProviderRuntimeLiveApply identifies when a runtime selection change sent to a live
// session_config process takes effect.
type ProviderRuntimeLiveApply string

const (
	// ProviderRuntimeLiveApplyNextStep means a running turn uses the change from the agent's next step.
	ProviderRuntimeLiveApplyNextStep ProviderRuntimeLiveApply = "next_step"
	// ProviderRuntimeLiveApplyNextTurn means the provider honors the change from its next turn.
	ProviderRuntimeLiveApplyNextTurn ProviderRuntimeLiveApply = "next_turn"
)

// SessionProviderOptionPayload is one workspace-visible session provider option.
type SessionProviderOptionPayload struct {
	Name            string                  `json:"name"`
	DisplayName     string                  `json:"display_name,omitempty"`
	Harness         string                  `json:"harness,omitempty"`
	RuntimeProvider string                  `json:"runtime_provider,omitempty"`
	RuntimeStrategy ProviderRuntimeStrategy `json:"runtime_strategy,omitempty"`
	// RuntimeLiveApply is omitted when runtime changes wait for the next prompt's process.
	RuntimeLiveApply ProviderRuntimeLiveApply `json:"runtime_live_apply,omitempty"`
	AuthMode         string                   `json:"auth_mode,omitempty"`
	EnvPolicy        string                   `json:"env_policy,omitempty"`
	HomePolicy       string                   `json:"home_policy,omitempty"`
}
