package contract

// ProviderRuntimeStrategy identifies how a provider applies runtime controls.
type ProviderRuntimeStrategy string

const (
	ProviderRuntimeStrategySessionConfig   ProviderRuntimeStrategy = "session_config"
	ProviderRuntimeStrategyLaunchArg       ProviderRuntimeStrategy = "launch_arg"
	ProviderRuntimeStrategyProviderManaged ProviderRuntimeStrategy = "provider_managed"
)

// SessionProviderOptionPayload is one workspace-visible session provider option.
type SessionProviderOptionPayload struct {
	Name            string                  `json:"name"`
	DisplayName     string                  `json:"display_name,omitempty"`
	Harness         string                  `json:"harness,omitempty"`
	RuntimeProvider string                  `json:"runtime_provider,omitempty"`
	RuntimeStrategy ProviderRuntimeStrategy `json:"runtime_strategy,omitempty"`
	AuthMode        string                  `json:"auth_mode,omitempty"`
	EnvPolicy       string                  `json:"env_policy,omitempty"`
	HomePolicy      string                  `json:"home_policy,omitempty"`
}
