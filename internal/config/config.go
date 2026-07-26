package config

import (
	"errors"

	"strings"
)

const (
	configCLIKey                             = "cli"
	configDefaultsAgentPath                  = "defaults.agent"
	configDreamingKey                        = "dreaming"
	configExtractorKey                       = "extractor"
	configExtractorModePostMessage           = "post_message"
	configHybridKey                          = "hybrid"
	configLLMKey                             = "llm"
	configJsonlKey                           = "jsonl"
	configExtractorQueueCoalesceMaxPath      = "memory.extractor.queue.coalesce_max"
	configMemoryRecallWeightsBm25UnicodePath = "memory.recall.weights.bm25_unicode"
	configProviderKey                        = "provider"
	configToolKey                            = "tool"
	configUDSKey                             = "uds"
)

const (
	// DirName is the AGH directory name used for both the global home and workspace overlays.
	DirName = ".agh"
	// ConfigName is the standard TOML configuration filename.
	ConfigName = "config.toml"
	// marketplaceSchemeHTTP is the accepted plaintext marketplace URL scheme.
	marketplaceSchemeHTTP = "http"
	urlSchemeHTTPS        = "https"
	// skillsMarketplaceRegistryClawhub is the currently supported skills marketplace registry.
	skillsMarketplaceRegistryClawhub = "clawhub"
)

const defaultMemoryWorkspaceTOMLPath = "<workspace>/.agh/workspace.toml"

// ErrSandboxProfileNotFound reports a sandbox profile reference that is not configured.
var ErrSandboxProfileNotFound = errors.New("sandbox profile not found")

// DefaultsConfig holds global runtime defaults.
type DefaultsConfig struct {
	Agent    string `toml:"agent"`
	Provider string `toml:"provider,omitempty"`
	Sandbox  string `toml:"sandbox,omitempty"`
}

// ValidationError preserves the config path for agent-parseable validation failures.
type ValidationError struct {
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
