package settings

import (
	aghconfig "github.com/compozy/agh/internal/config"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/providerauth"
)

// ProviderSettings is the editable provider overlay payload.
type ProviderSettings struct {
	Command         string
	DisplayName     string
	Models          aghconfig.ProviderModelsConfig
	ModelsSet       bool
	Harness         aghconfig.ProviderHarness
	RuntimeProvider string
	Transport       string
	BaseURL         string
	AuthMode        aghconfig.ProviderAuthMode
	EnvPolicy       aghconfig.ProviderEnvPolicy
	HomePolicy      aghconfig.ProviderHomePolicy
	AuthStatusCmd   string
	AuthLoginCmd    string
	CredentialSlots []aghconfig.ProviderCredentialSlot
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

// ProviderNativeCLIStatus is a redacted provider-owned CLI availability diagnostic.
type ProviderNativeCLIStatus = providerauth.NativeCLIStatus

// ProviderAuthStatus is a redacted provider authentication readiness summary.
type ProviderAuthStatus struct {
	Mode       aghconfig.ProviderAuthMode
	EnvPolicy  aghconfig.ProviderEnvPolicy
	HomePolicy aghconfig.ProviderHomePolicy
	State      string
	Code       string
	Message    string
	StatusCmd  string
	LoginCmd   string
	LoginEnv   []string
	NativeCLI  *ProviderNativeCLIStatus
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
	Profile             aghconfig.SandboxProfile
	WorkspaceUsageCount int
	SourceMetadata      SourceMetadata
}

// HookItem is one config-defined hook collection row.
type HookItem struct {
	Name           string
	Declaration    hookspkg.HookDecl
	SourceMetadata SourceMetadata
}
