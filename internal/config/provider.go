package config

import (
	"errors"

	"regexp"
)

const (
	providerAnthropicKey = "anthropic"
	providerClaudeKey    = "claude"
)

const (
	providerMiniMaxM21Path             = "MiniMax-M2.1"
	providerNodeOptionsValue           = "NODE_OPTIONS"
	providerAnthropicClaudeOpus47Path  = "anthropic/claude-opus-4-7"
	providerBlackboxKey                = "blackbox"
	providerClaudeCodeAlias            = "claude-code"
	providerGeminiKey                  = "gemini"
	modelClaudeOpus47ID                = "claude-opus-4-7"
	providerClineKey                   = "cline"
	providerCodexKey                   = "codex"
	providerDevstralMediumLatestValue  = "devstral-medium-latest"
	providerGemini31ProPreviewPath     = "gemini-3.1-pro-preview"
	providerGlm46Path                  = "glm-4.6"
	providerGooseKey                   = "goose"
	providerGrokAlias                  = "grok"
	providerGrok4FastNonReasoningValue = "grok-4-fast-non-reasoning"
	providerGroqKey                    = "groq"
	providerHermesKey                  = "hermes"
	providerHighKey                    = "high"
	providerJunieKey                   = "junie"
	providerKimiAlias                  = "kimi"
	providerKimiCLIValue               = "kimi-cli"
	providerKimiCodingValue            = "kimi-coding"
	providerKimiK2ThinkingValue        = "kimi-k2-thinking"
	providerMediumKey                  = "medium"
	providerMinimaxKey                 = "minimax"
	providerMistralKey                 = "mistral"
	providerMoonshotKey                = "moonshot"
	claudeProviderCommand              = "npx -y @agentclientprotocol/claude-agent-acp@latest"
	providerOpenaiGpt54Path            = "openai/gpt-5.4"
	providerOpenaiGptOss120bPath       = "openai/gpt-oss-120b"
	providerOpenclawKey                = "openclaw"
	providerOpencodeKey                = "opencode"
	providerOpenhandsKey               = "openhands"
	providerOpenrouterKey              = "openrouter"
	providerQoderKey                   = "qoder"
	providerQwenAlias                  = "qwen"
	providerQwenCodeValue              = "qwen-code"
	providerQwen36PlusPath             = "qwen3.6-plus"
	providerVercelAIGatewayValue       = "vercel-ai-gateway"
	providerXaiKey                     = "xai"
	providerXaiDotAlias                = "x.ai"
	providerZaiKey                     = "zai"
)

var providerSecretRefSegmentPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*$`)

// ProviderHarness identifies the runtime strategy used to launch a provider.
type ProviderHarness string

const (
	// ProviderHarnessACP launches the configured command directly as an ACP runtime.
	ProviderHarnessACP ProviderHarness = "acp"
	// ProviderHarnessPiACP launches pi through the pi-acp adapter and materializes provider settings.
	ProviderHarnessPiACP ProviderHarness = "pi_acp"
)

// ProviderAuthMode identifies who owns launch-time provider authentication.
type ProviderAuthMode string

const (
	// ProviderAuthModeNativeCLI lets the provider CLI use its own login/session state.
	ProviderAuthModeNativeCLI ProviderAuthMode = "native_cli"
	// ProviderAuthModeBoundSecret injects explicitly configured credential slots at launch.
	ProviderAuthModeBoundSecret ProviderAuthMode = "bound_secret"
	// ProviderAuthModeNone launches the provider without AGH-managed credentials.
	ProviderAuthModeNone ProviderAuthMode = "none"
)

// ProviderEnvPolicy identifies which daemon environment is inherited by a provider process.
type ProviderEnvPolicy string

const (
	// ProviderEnvPolicyFiltered removes secret-shaped daemon variables but keeps operator context.
	ProviderEnvPolicyFiltered ProviderEnvPolicy = "filtered"
	// ProviderEnvPolicyIsolated keeps only a fixed operational allowlist.
	ProviderEnvPolicyIsolated ProviderEnvPolicy = "isolated"
)

// ProviderHomePolicy identifies whether provider CLI state comes from the operator home or an isolated home.
type ProviderHomePolicy string

const (
	// ProviderHomePolicyOperator lets native CLIs read their existing operator login state.
	ProviderHomePolicyOperator ProviderHomePolicy = "operator"
	// ProviderHomePolicyIsolated points native CLIs at an AGH-owned provider home.
	ProviderHomePolicyIsolated ProviderHomePolicy = "isolated"
)

// ProviderNoneSecurity identifies why auth_mode=none is safe for a provider.
type ProviderNoneSecurity string

const (
	// ProviderNoneSecurityLocalTransport limits unauthenticated providers to local transport.
	ProviderNoneSecurityLocalTransport ProviderNoneSecurity = "local_transport"
	// ProviderNoneSecurityExternalIdentity delegates authentication to the provider transport.
	ProviderNoneSecurityExternalIdentity ProviderNoneSecurity = "external_identity"
	// ProviderNoneSecurityPublicReadonly permits unauthenticated public read-only providers.
	ProviderNoneSecurityPublicReadonly ProviderNoneSecurity = "public_readonly"
)

// ProviderCredentialSlot describes one launch-time secret binding needed by a provider.
type ProviderCredentialSlot struct {
	Name      string `toml:"name"`
	TargetEnv string `toml:"target_env"`
	SecretRef string `toml:"secret_ref"`
	Kind      string `toml:"kind,omitempty"`
	Required  bool   `toml:"required"`
}

// ModelCatalogConfig controls daemon-owned model catalog sources.
type ModelCatalogConfig struct {
	Sources ModelCatalogSourcesConfig `toml:"sources,omitempty"`
}

// ModelCatalogSourcesConfig groups built-in model catalog sources.
type ModelCatalogSourcesConfig struct {
	ModelsDev ModelsDevSourceConfig `toml:"models_dev,omitempty"`
}

// ModelsDevSourceConfig controls the models.dev catalog source.
type ModelsDevSourceConfig struct {
	Enabled  *bool  `toml:"enabled,omitempty"`
	Endpoint string `toml:"endpoint,omitempty"`
	TTL      string `toml:"ttl,omitempty"`
	Timeout  string `toml:"timeout,omitempty"`
}

// ProviderConfig describes how to launch a provider in ACP mode.
type ProviderConfig struct {
	Command         string                   `toml:"command"`
	DisplayName     string                   `toml:"display_name,omitempty"`
	Models          ProviderModelsConfig     `toml:"models,omitempty"`
	Harness         ProviderHarness          `toml:"harness,omitempty"`
	RuntimeProvider string                   `toml:"runtime_provider,omitempty"`
	Transport       string                   `toml:"transport,omitempty"`
	BaseURL         string                   `toml:"base_url,omitempty"`
	AuthMode        ProviderAuthMode         `toml:"auth_mode,omitempty"`
	EnvPolicy       ProviderEnvPolicy        `toml:"env_policy,omitempty"`
	HomePolicy      ProviderHomePolicy       `toml:"home_policy,omitempty"`
	NoneSecurity    ProviderNoneSecurity     `toml:"none_security,omitempty"`
	AuthStatusCmd   string                   `toml:"auth_status_command,omitempty"`
	AuthLoginCmd    string                   `toml:"auth_login_command,omitempty"`
	SessionMCP      *bool                    `toml:"session_mcp,omitempty"`
	CredentialSlots []ProviderCredentialSlot `toml:"credential_slots,omitempty"`
	MCPServers      []MCPServer              `toml:"mcp_servers,omitempty"`
}

// MCPServerTransport identifies how AGH reaches an MCP server.
type MCPServerTransport string

const (
	// MCPServerTransportStdio launches a local subprocess and talks MCP over stdio.
	MCPServerTransportStdio MCPServerTransport = "stdio"
	// MCPServerTransportHTTP talks to a remote streamable HTTP MCP endpoint.
	MCPServerTransportHTTP MCPServerTransport = "http"
	// MCPServerTransportSSE talks to a remote SSE MCP endpoint.
	MCPServerTransportSSE MCPServerTransport = "sse"
)

// MCPAuthType identifies the remote MCP authentication mechanism.
type MCPAuthType string

const (
	// MCPAuthTypeOAuth2PKCE uses OAuth 2.1 authorization code with PKCE.
	MCPAuthTypeOAuth2PKCE MCPAuthType = "oauth2_pkce"
)

// MCPAuthConfig describes remote MCP OAuth configuration. It stores endpoint
// metadata and secret refs only; token material is persisted through the
// vault-backed auth token store.
type MCPAuthConfig struct {
	Type             MCPAuthType `json:"type,omitempty"              yaml:"type,omitempty"              toml:"type,omitempty"`
	IssuerURL        string      `json:"issuer_url,omitempty"        yaml:"issuer_url,omitempty"        toml:"issuer_url,omitempty"`
	MetadataURL      string      `json:"metadata_url,omitempty"      yaml:"metadata_url,omitempty"      toml:"metadata_url,omitempty"`
	AuthorizationURL string      `json:"authorization_url,omitempty" yaml:"authorization_url,omitempty" toml:"authorization_url,omitempty"`
	TokenURL         string      `json:"token_url,omitempty"         yaml:"token_url,omitempty"         toml:"token_url,omitempty"`
	RevocationURL    string      `json:"revocation_url,omitempty"    yaml:"revocation_url,omitempty"    toml:"revocation_url,omitempty"`
	ClientID         string      `json:"client_id,omitempty"         yaml:"client_id,omitempty"         toml:"client_id,omitempty"`
	ClientSecretRef  string      `json:"client_secret_ref,omitempty" yaml:"client_secret_ref,omitempty" toml:"client_secret_ref,omitempty"`
	Scopes           []string    `json:"scopes,omitempty"            yaml:"scopes,omitempty"            toml:"scopes,omitempty"`
}

// ResolvedAgent is the effective runtime configuration for a parsed agent definition.
type ResolvedAgent struct {
	Name            string
	Provider        string
	Command         string
	DisplayName     string
	Model           string
	ReasoningEffort string
	Tools           []string
	Toolsets        []string
	DenyTools       []string
	Permissions     string
	Harness         ProviderHarness
	RuntimeProvider string
	Transport       string
	BaseURL         string
	AuthMode        ProviderAuthMode
	EnvPolicy       ProviderEnvPolicy
	HomePolicy      ProviderHomePolicy
	NoneSecurity    ProviderNoneSecurity
	AuthStatusCmd   string
	AuthLoginCmd    string
	SessionMCP      bool
	CredentialSlots []ProviderCredentialSlot
	MCPServers      []MCPServer
	Reasoning       ProviderReasoningConfig
	RuntimeSources  ResolvedRuntimeSources
	Prompt          string
}

// ErrProviderUnavailable reports that a requested provider cannot be resolved
// from the effective workspace/global config.
var ErrProviderUnavailable = errors.New("provider unavailable")

const (
	piACPCommand             = "npx -y pi-acp@latest"
	piACPAuthLoginCommand    = piACPCommand + " --terminal-login"
	providerAPIKeyCredential = "api_key"
	defaultModelsDevEndpoint = "https://models.dev/api.json"
	defaultModelsDevTTL      = "24h"
	defaultModelsDevTimeout  = "10s"
)

var builtinProviderAliases = map[string]string{
	"blackbox-ai":                providerBlackboxKey,
	"blackboxai":                 providerBlackboxKey,
	providerClaudeCodeAlias:      providerClaudeKey,
	"cline-cli":                  providerClineKey,
	"goose-cli":                  providerGooseKey,
	"hermes-agent":               providerHermesKey,
	"junie-cli":                  providerJunieKey,
	"ai-gateway":                 providerVercelAIGatewayValue,
	"aigateway":                  providerVercelAIGatewayValue,
	providerKimiAlias:            providerMoonshotKey,
	"kimi cli":                   providerKimiCLIValue,
	providerKimiCLIValue:         providerKimiCLIValue,
	"kimi-code":                  providerKimiCLIValue,
	providerKimiCodingValue:      providerMoonshotKey,
	"moonshotai":                 providerMoonshotKey,
	"open-hands":                 providerOpenhandsKey,
	"openhands-cli":              providerOpenhandsKey,
	"openclaw-cli":               providerOpenclawKey,
	"open-code":                  providerOpencodeKey,
	"opencode-ai":                providerOpencodeKey,
	"qoder-cli":                  providerQoderKey,
	providerQwenAlias:            providerQwenCodeValue,
	"qwen cli":                   providerQwenCodeValue,
	"qwen code":                  providerQwenCodeValue,
	providerQwenCodeValue:        providerQwenCodeValue,
	"vercel":                     providerVercelAIGatewayValue,
	"vercel-gateway":             providerVercelAIGatewayValue,
	"vercel-ai":                  providerVercelAIGatewayValue,
	providerVercelAIGatewayValue: providerVercelAIGatewayValue,
	"z.ai":                       providerZaiKey,
	"z-ai":                       providerZaiKey,
	"z_ai":                       providerZaiKey,
	"glm":                        providerZaiKey,
	"openrouter-ai":              providerOpenrouterKey,
	"openrouter-gateway":         providerOpenrouterKey,
	"minimax-ai":                 providerMinimaxKey,
	"minimax-cn":                 providerMinimaxKey,
	providerGrokAlias:            providerXaiKey,
	"x-ai":                       providerXaiKey,
	providerXaiDotAlias:          providerXaiKey,
	"mistralai":                  providerMistralKey,
	"mistral-ai":                 providerMistralKey,
}
