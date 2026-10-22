package config

import (
	"strings"
)

const (
	configCLIKey            = "cli"
	configDefaultsAgentPath = "defaults.agent"
	// configDefaultsModelPath and configDefaultsReasoningEffortPath name the
	// session-runtime refinements of defaults.provider.
	configDefaultsModelPath           = "defaults.model"
	configDefaultsReasoningEffortPath = "defaults.reasoning_effort"
	configProviderKey                 = "provider"
	configToolKey                     = "tool"
	configUDSKey                      = "uds"
)

const (
	// DirName is the CompozyOS directory name used for both the global home and workspace overlays.
	DirName = ".compozy"
	// ConfigName is the standard TOML configuration filename.
	ConfigName = "config.toml"
	// marketplaceSchemeHTTP is the accepted plaintext marketplace URL scheme.
	marketplaceSchemeHTTP = "http"
	urlSchemeHTTPS        = "https"
)

// DefaultsConfig holds global runtime defaults.
//
// Model and ReasoningEffort refine the runtime of sessions whose provider
// resolves from Provider. They never apply to an agent-authored provider, and
// a config layer that changes Provider drops the values inherited from lower
// layers, so a model chosen for one provider cannot reach another.
type DefaultsConfig struct {
	Agent           string `toml:"agent"`
	Provider        string `toml:"provider,omitempty"`
	Model           string `toml:"model,omitempty"`
	ReasoningEffort string `toml:"reasoning_effort,omitempty"`
}

// ValidationError preserves the config path for agent-parseable validation failures.
type ValidationError struct {
	Code    string
	Path    string
	Message string
}

func (e ValidationError) Error() string {
	if strings.TrimSpace(e.Path) == "" {
		return e.Message
	}
	if strings.TrimSpace(e.Message) == "" {
		return e.Path
	}
	return e.Path + " " + e.Message
}

// UserScopeConfig controls the execution directory for profile-owned work.
type UserScopeConfig struct {
	WorkDir string `toml:"work_dir"`
}
