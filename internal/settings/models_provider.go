package settings

import (
	compozyconfig "github.com/compozy/compozy/internal/config"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	authproviders "github.com/compozy/compozy/internal/providers"
)

// ProviderSettings is the editable provider overlay payload.
type ProviderSettings struct {
	SteerCapability compozyconfig.SteerCapability
	Command         string
	DisplayName     string
	Models          compozyconfig.ProviderModelsConfig
	ModelsSet       bool
	Harness         compozyconfig.ProviderHarness
	RuntimeProvider string
	Transport       string
	BaseURL         string
	AuthMode        compozyconfig.ProviderAuthMode
	EnvPolicy       compozyconfig.ProviderEnvPolicy
	HomePolicy      compozyconfig.ProviderHomePolicy
	AuthStatusCmd   string
	AuthLoginCmd    string
	AuthLoginCmdSet bool
	CredentialSlots []compozyconfig.ProviderCredentialSlot
}

// ProviderCredentialStatus is a redacted launch credential status.
type ProviderCredentialStatus struct {
	Name      string
	TargetEnv string
	SecretRef string
	Kind      string
	Required  bool
	Present   bool
	Source    string
}

// ProviderAuthStatus is a redacted provider authentication readiness summary.
type ProviderAuthStatus struct {
	Mode       compozyconfig.ProviderAuthMode
	EnvPolicy  compozyconfig.ProviderEnvPolicy
	HomePolicy compozyconfig.ProviderHomePolicy
	State      string
	Code       string
	Message    string
	StatusCmd  string
	Login      authproviders.ProviderLoginDescriptor
}

// ProviderSecretWrite is one write-only provider secret mutation.
type ProviderSecretWrite struct {
	Name      string
	SecretRef string
	Kind      string
	Value     string
}

// ProviderFallback reports the builtin provider revealed when an overlay is removed.
type ProviderFallback struct {
	Source   SourceRef
	Settings ProviderSettings
}

// ProviderItem is one provider collection row.
type ProviderItem struct {
	Name             string
	Settings         ProviderSettings
	Default          bool
	CommandAvailable bool
	Credentials      []ProviderCredentialStatus
	AuthStatus       ProviderAuthStatus
	SourceMetadata   SourceMetadata
	Fallback         *ProviderFallback
}

// SandboxItem is one sandbox collection row.
type SandboxItem struct {
	Name                string
	Profile             compozyconfig.SandboxProfile
	WorkspaceUsageCount int
	SourceMetadata      SourceMetadata
}

// HookItem is one config-defined hook collection row.
type HookItem struct {
	Name           string
	Declaration    hookspkg.HookDecl
	SourceMetadata SourceMetadata
}
