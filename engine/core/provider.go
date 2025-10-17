package core

import (
	"encoding/json"

	appconfig "github.com/compozy/compozy/pkg/config"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"
)

// ProviderName represents the name of a supported LLM provider in the Compozy ecosystem.
// Each provider name corresponds to a specific AI service that can be used for workflow execution.
type ProviderName string

const (
	ProviderOpenAI     ProviderName = "openai"     // OpenAI GPT models (GPT-4, GPT-3.5, etc.)
	ProviderGroq       ProviderName = "groq"       // Groq fast inference platform
	ProviderAnthropic  ProviderName = "anthropic"  // Anthropic Claude models
	ProviderGoogle     ProviderName = "google"     // Google Gemini models
	ProviderOllama     ProviderName = "ollama"     // Ollama local model hosting
	ProviderDeepSeek   ProviderName = "deepseek"   // DeepSeek AI models
	ProviderXAI        ProviderName = "xai"        // xAI Grok models
	ProviderCerebras   ProviderName = "cerebras"   // Cerebras fast inference platform
	ProviderOpenRouter ProviderName = "openrouter" // OpenRouter multi-model gateway
	ProviderMock       ProviderName = "mock"       // Mock provider for testing
)

// SupportsNativeJSONSchema reports whether the provider accepts OpenAI-compatible
// response_format JSON schema requests. Providers like Groq currently reject
// json_schema payloads, so they return false here to force prompt-based fallbacks.
func SupportsNativeJSONSchema(provider ProviderName) bool {
	switch provider {
	case ProviderOpenAI, ProviderXAI, ProviderCerebras:
		return true
	default:
		return false
	}
}

// PromptParams defines the parameters that control LLM behavior during text generation.
// These parameters allow fine-tuning of model responses for specific use cases and requirements.
//
// **Usage in Compozy:**
//   - Applied to agent configurations for consistent behavior across workflow tasks
//   - Used in task-specific overrides for specialized generation requirements
//   - Converted to provider-specific parameters during LLM client creation
//   - Support template processing for dynamic parameter adjustment
//
// **Parameter Effects:**
//   - **Temperature**: Controls randomness and creativity (0.0 = deterministic, 1.0 = very creative)
//   - **MaxTokens**: Limits response length to prevent excessive generation costs
//   - **StopWords**: Provides early termination triggers for specific content patterns
//   - **TopK/TopP**: Fine-tune sampling behavior for response quality
//   - **Seed**: Enables reproducible outputs for testing and consistency
//   - **RepetitionPenalty**: Reduces repetitive content in longer responses
//
// **Example Configuration:**
//
// ```yaml
//
// models:
//   - provider: openai
//     model: gpt-4-turbo
//     params:
//     temperature: 0.7
//     max_tokens: 4000
//     stop_words: ["END", "STOP"]
//     top_p: 0.9
//     seed: 42
//     repetition_penalty: 1.1
//
// ```
type PromptParams struct {
	// MaxTokens limits the maximum number of tokens in the generated response.
	// This parameter is crucial for cost control and response time management.
	// - **Range**: 1 to model-specific maximum (e.g., 8192 for GPT-4)
	// - **Default**: Provider-specific default (typically 1000-2000)
	MaxTokens int32 `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty" mapstructure:"max_tokens,omitempty"`

	// Temperature controls the randomness of the generated text.
	// Lower values produce more deterministic, focused responses.
	// Higher values increase creativity and variation but may reduce coherence.
	// - **Range**: 0.0 (deterministic) to 1.0 (maximum randomness)
	// - **Recommended**: 0.1-0.3 for factual tasks, 0.7-0.9 for creative tasks
	Temperature float64 `json:"temperature,omitempty" yaml:"temperature,omitempty" mapstructure:"temperature,omitempty"`

	// StopWords defines a list of strings that will halt text generation when encountered.
	// Useful for creating structured outputs or preventing unwanted content patterns.
	//
	// - **Example**: `["END", "STOP", "\n\n---"]` for section-based content
	// > **Note:**: Not all providers support stop words; check provider documentation
	StopWords []string `json:"stop_words,omitempty" yaml:"stop_words,omitempty" mapstructure:"stop_words,omitempty"`

	// TopK limits the number of highest probability tokens considered during sampling.
	// Lower values focus on the most likely tokens, higher values allow more variety.
	// - **Range**: 1 to vocabulary size (typically 1-100)
	// - **Provider Support**: Primarily Google models and some local models
	TopK int `json:"top_k,omitempty" yaml:"top_k,omitempty" mapstructure:"top_k,omitempty"`

	// TopP (nucleus sampling) considers only tokens with cumulative probability up to this value.
	// Dynamically adjusts the vocabulary size based on probability distribution.
	// - **Range**: 0.0 to 1.0
	// - **Recommended**: 0.9 for balanced outputs, 0.95 for more variety
	TopP float64 `json:"top_p,omitempty" yaml:"top_p,omitempty" mapstructure:"top_p,omitempty"`

	// Seed provides a random seed for reproducible outputs.
	// When set, the same input with the same parameters will generate identical responses.
	// - **Use Cases**: Testing, debugging, demonstration, A/B testing
	// > **Note:**: Not all providers support seeding; OpenAI and some others do
	Seed int `json:"seed,omitempty" yaml:"seed,omitempty" mapstructure:"seed,omitempty"`

	// MinLength specifies the minimum number of tokens that must be generated.
	// Prevents the model from generating responses that are too short.
	// - **Range**: 1 to MaxTokens
	// - **Provider Support**: Limited; primarily local models
	MinLength int `json:"min_length,omitempty" yaml:"min_length,omitempty" mapstructure:"min_length,omitempty"`

	// MaxLength provides an alternative way to specify maximum response length.
	// Typically used by providers that distinguish between length and token limits.
	// - **Range**: MinLength to provider-specific maximum
	// - **Provider Support**: Primarily local models and some API providers
	MaxLength int `json:"max_length,omitempty" yaml:"max_length,omitempty" mapstructure:"max_length,omitempty"`

	// RepetitionPenalty reduces the likelihood of repeating the same tokens.
	// Values > 1.0 penalize repetition, values < 1.0 encourage it.
	// - **Range**: 0.1 to 2.0
	// - **Recommended**: 1.0 (no penalty) to 1.2 (moderate penalty)
	// - **Provider Support**: Primarily local models (Ollama, etc.)
	RepetitionPenalty float64 `json:"repetition_penalty,omitempty" yaml:"repetition_penalty,omitempty" mapstructure:"repetition_penalty,omitempty"`
	// FrequencyPenalty penalizes tokens based on their frequency in the text so far.
	// Positive values reduce repetition, negative values encourage it.
	// - **Range**: -2.0 to 2.0
	// - **Recommended**: 0.0 (no penalty) to 0.5 (moderate penalty)
	// - **Provider Support**: OpenAI, Groq
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"  yaml:"frequency_penalty,omitempty"  mapstructure:"frequency_penalty,omitempty"`
	// PresencePenalty penalizes tokens that have already appeared in the text.
	// Positive values encourage the model to talk about new topics.
	// - **Range**: -2.0 to 2.0
	// - **Recommended**: 0.0 (no penalty) to 0.5 (moderate penalty)
	// - **Provider Support**: OpenAI, Groq
	PresencePenalty float64 `json:"presence_penalty,omitempty"   yaml:"presence_penalty,omitempty"   mapstructure:"presence_penalty,omitempty"`
	// N specifies how many completion choices to generate for each prompt.
	// Useful for generating multiple alternatives and selecting the best one.
	// - **Range**: 1 to provider-specific maximum (typically 1-10)
	// - **Default**: 1
	// - **Provider Support**: OpenAI
	N int `json:"n,omitempty"                  yaml:"n,omitempty"                  mapstructure:"n,omitempty"`
	// CandidateCount specifies the number of response candidates to generate.
	// Similar to N but used by Google AI models.
	// - **Range**: 1 to provider-specific maximum
	// - **Provider Support**: Google AI (Gemini)
	CandidateCount int `json:"candidate_count,omitempty"    yaml:"candidate_count,omitempty"    mapstructure:"candidate_count,omitempty"`
	// Metadata contains backend-specific parameters not covered by standard fields.
	// Use this for provider-specific features or experimental parameters.
	// - **Example**: Custom headers, request tracking, A/B test variants
	// - **Provider Support**: Varies by provider
	Metadata map[string]any `json:"metadata,omitempty"           yaml:"metadata,omitempty"           mapstructure:"metadata,omitempty"`
	// internal presence flags (not serialized)
	_set ppSet `json:"-"                            yaml:"-"`
}

// ppSet tracks which YAML keys were explicitly present for PromptParams.
// This enables precise merge semantics (distinguish unset vs explicit zero).
type ppSet struct {
	MaxTokens         bool
	Temperature       bool
	StopWords         bool
	TopK              bool
	TopP              bool
	Seed              bool
	MinLength         bool
	RepetitionPenalty bool
	FrequencyPenalty  bool
	PresencePenalty   bool
	N                 bool
	CandidateCount    bool
	Metadata          bool
}

// buildPresenceFlags extracts which YAML keys were explicitly provided
func buildPresenceFlags(raw map[string]any) ppSet {
	flags := ppSet{}
	for k := range raw {
		switch k {
		case "max_tokens":
			flags.MaxTokens = true
		case "temperature":
			flags.Temperature = true
		case "stop_words":
			flags.StopWords = true
		case "top_k":
			flags.TopK = true
		case "top_p":
			flags.TopP = true
		case "seed":
			flags.Seed = true
		case "min_length":
			flags.MinLength = true
		case "repetition_penalty":
			flags.RepetitionPenalty = true
		case "frequency_penalty":
			flags.FrequencyPenalty = true
		case "presence_penalty":
			flags.PresencePenalty = true
		case "n":
			flags.N = true
		case "candidate_count":
			flags.CandidateCount = true
		case "metadata":
			flags.Metadata = true
		}
	}
	return flags
}

// UnmarshalYAML records which keys are present, then decodes into the struct.
// This preserves intent for zero values during merges.
func (p *PromptParams) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	// Track key presence
	var raw map[string]any
	if err := value.Decode(&raw); err != nil {
		return err
	}
	flags := buildPresenceFlags(raw)
	type alias PromptParams
	var tmp alias
	if err := value.Decode(&tmp); err != nil {
		return err
	}
	*p = PromptParams(tmp)
	p._set = flags
	return nil
}

// Presence helpers (exported via methods) for cross-package checks
func (p *PromptParams) IsSetMaxTokens() bool         { return p._set.MaxTokens }
func (p *PromptParams) IsSetTemperature() bool       { return p._set.Temperature }
func (p *PromptParams) IsSetStopWords() bool         { return p._set.StopWords }
func (p *PromptParams) IsSetTopK() bool              { return p._set.TopK }
func (p *PromptParams) IsSetTopP() bool              { return p._set.TopP }
func (p *PromptParams) IsSetSeed() bool              { return p._set.Seed }
func (p *PromptParams) IsSetMinLength() bool         { return p._set.MinLength }
func (p *PromptParams) IsSetRepetitionPenalty() bool { return p._set.RepetitionPenalty }
func (p *PromptParams) IsSetFrequencyPenalty() bool  { return p._set.FrequencyPenalty }
func (p *PromptParams) IsSetPresencePenalty() bool   { return p._set.PresencePenalty }
func (p *PromptParams) IsSetN() bool                 { return p._set.N }
func (p *PromptParams) IsSetCandidateCount() bool    { return p._set.CandidateCount }
func (p *PromptParams) IsSetMetadata() bool          { return p._set.Metadata }

// SetMaxTokens sets MaxTokens and records its explicit configuration.
func (p *PromptParams) SetMaxTokens(value int32) {
	p.MaxTokens = value
	p._set.MaxTokens = true
}

// SetTemperature sets Temperature and records its explicit configuration.
func (p *PromptParams) SetTemperature(value float64) {
	p.Temperature = value
	p._set.Temperature = true
}

// SetStopWords copies the provided stop words and records their presence.
func (p *PromptParams) SetStopWords(words []string) {
	if len(words) == 0 {
		p.StopWords = nil
		p._set.StopWords = true
		return
	}
	p.StopWords = append([]string(nil), words...)
	p._set.StopWords = true
}

// SetTopK sets TopK and records its explicit configuration.
func (p *PromptParams) SetTopK(value int) {
	p.TopK = value
	p._set.TopK = true
}

// SetTopP sets TopP and records its explicit configuration.
func (p *PromptParams) SetTopP(value float64) {
	p.TopP = value
	p._set.TopP = true
}

// SetSeed sets Seed and records its explicit configuration.
func (p *PromptParams) SetSeed(value int) {
	p.Seed = value
	p._set.Seed = true
}

// SetMinLength sets MinLength and records its explicit configuration.
func (p *PromptParams) SetMinLength(value int) {
	p.MinLength = value
	p._set.MinLength = true
}

// SetRepetitionPenalty sets RepetitionPenalty and records its explicit configuration.
func (p *PromptParams) SetRepetitionPenalty(value float64) {
	p.RepetitionPenalty = value
	p._set.RepetitionPenalty = true
}

// SetFrequencyPenalty sets FrequencyPenalty and records its explicit configuration.
func (p *PromptParams) SetFrequencyPenalty(value float64) {
	p.FrequencyPenalty = value
	p._set.FrequencyPenalty = true
}

// SetPresencePenalty sets PresencePenalty and records its explicit configuration.
func (p *PromptParams) SetPresencePenalty(value float64) {
	p.PresencePenalty = value
	p._set.PresencePenalty = true
}

// SetN sets N and records its explicit configuration.
func (p *PromptParams) SetN(value int) {
	p.N = value
	p._set.N = true
}

// SetCandidateCount sets CandidateCount and records its explicit configuration.
func (p *PromptParams) SetCandidateCount(value int) {
	p.CandidateCount = value
	p._set.CandidateCount = true
}

// SetMetadata copies the provided metadata map and records its presence.
func (p *PromptParams) SetMetadata(metadata map[string]any) {
	if len(metadata) == 0 {
		p.Metadata = nil
		p._set.Metadata = true
		return
	}
	p.Metadata = make(map[string]any, len(metadata))
	for k, v := range metadata {
		p.Metadata[k] = v
	}
	p._set.Metadata = true
}

// ProviderConfig represents the complete configuration for an LLM provider in Compozy workflows.
// This configuration defines how agents connect to and interact with specific AI services.
//
// **Core Purpose:**
//   - Establishes connection parameters for LLM providers
//   - Defines model-specific settings and authentication
//   - Controls generation behavior through prompt parameters
//   - Enables multi-provider workflows with consistent configuration
//
// > **Security Note:** Always use environment variables or secure secret management
// > for API keys. Never hardcode credentials in configuration files.
type ProviderConfig struct {
	// Provider specifies which AI service to use for LLM operations.
	// Must match one of the supported ProviderName constants.
	//
	// - **Examples**: `"openai"`, `"anthropic"`, `"google"`, `"ollama"`
	Provider ProviderName `json:"provider" yaml:"provider" mapstructure:"provider"`

	// Model defines the specific model identifier to use with the provider.
	// Model names are provider-specific and determine capabilities and pricing.
	//
	// - **Examples**:
	//   - OpenAI: `"gpt-4-turbo"`, `"gpt-3.5-turbo"`
	//   - Anthropic: `"claude-4-opus"`, `"claude-3-5-haiku-latest"`
	//   - Google: `"gemini-pro"`, `"gemini-pro-vision"`
	//   - Ollama: `"llama2:13b"`, `"mistral:7b"`
	Model string `json:"model" yaml:"model" mapstructure:"model"`

	// APIKey contains the authentication key for the AI provider.
	//
	// - **Security**: Use template references to environment variables.
	// - **Examples**: `"{{ .env.OPENAI_API_KEY }}"`, `"{{ .secrets.ANTHROPIC_KEY }}"`
	// > **Note:**: Required for most cloud providers, optional for local providers
	APIKey string `json:"api_key" yaml:"api_key" mapstructure:"api_key"`

	// APIURL specifies a custom API endpoint for the provider.
	// **Use Cases**:
	//   - Local model hosting (Ollama, OpenAI-compatible servers)
	//   - Enterprise API gateways
	//   - Regional API endpoints
	//   - Custom proxy servers
	//
	// **Examples**: `"http://localhost:11434"`, `"https://api.openai.com/v1"`
	APIURL string `json:"api_url" yaml:"api_url" mapstructure:"api_url"`

	// Params contains the generation parameters that control LLM behavior.
	// These parameters are applied to all requests using this provider configuration.
	// Can be overridden at the task or action level for specific requirements.
	Params PromptParams `json:"params" yaml:"params" mapstructure:"params"`

	// Organization specifies the organization ID for providers that support it.
	// - **Primary Use**: OpenAI organization management for billing and access control
	//
	// - **Example**: `"org-123456789abcdef"`
	// > **Note:**: Optional for most providers
	Organization string `json:"organization" yaml:"organization" mapstructure:"organization"`

	// Default indicates that this model should be used as the fallback when no explicit
	// model configuration is provided at the task or agent level.
	//
	// **Behavior**:
	//   - Only one model per project can be marked as default
	//   - When set to true, this model will be used for tasks/agents without explicit model config
	//   - Validation ensures at most one default model per project
	//
	// **Example**:
	// ```yaml
	// models:
	//   - provider: openai
	//     model: gpt-4
	//     default: true  # This will be used by default
	// ```
	Default bool `json:"default,omitempty" yaml:"default,omitempty" mapstructure:"default"`

	// MaxToolIterations optionally caps the maximum number of tool-call iterations
	// during a single LLM request when tools are available.
	// When > 0, overrides the global default for this model; 0 uses the global default.
	MaxToolIterations int `json:"max_tool_iterations,omitempty" yaml:"max_tool_iterations,omitempty" mapstructure:"max_tool_iterations,omitempty" validate:"min=0"`

	// RateLimit overrides concurrency limits and queue size for this provider.
	// When omitted the orchestrator applies the global defaults.
	RateLimit *appconfig.ProviderRateLimitConfig `json:"rate_limit,omitempty" yaml:"rate_limit,omitempty" mapstructure:"rate_limit,omitempty"`

	// ContextWindow optionally overrides the provider's default context window size.
	// When > 0, this value replaces the provider's default ContextWindowTokens capability.
	// Useful for providers like OpenRouter that support multiple models with varying limits.
	// - **Example**: 200000 for Claude 3.5 Sonnet via OpenRouter
	// - **Default**: Uses provider's default when not specified or <= 0
	ContextWindow int `json:"context_window,omitempty" yaml:"context_window,omitempty" mapstructure:"context_window,omitempty" validate:"min=0"`
}

// NewProviderConfig creates a new ProviderConfig with the specified core parameters.
// This constructor provides a convenient way to create provider configurations programmatically.
//
// **Parameters:**
//   - provider: The AI provider to use (e.g., ProviderOpenAI, ProviderAnthropic)
//   - model: The specific model identifier for the provider
//   - apiKey: The authentication key (use template references for security)
//
// **Returns:** A configured ProviderConfig ready for use in agents and workflows
//
// **Example Usage:**
//
//	```go
//	config := NewProviderConfig(
//	    ProviderOpenAI,
//	    "gpt-4-turbo",
//	    "{{ .env.OPENAI_API_KEY }}",
//	)
//	```
func NewProviderConfig(provider ProviderName, model string, apiKey string) *ProviderConfig {
	config := &ProviderConfig{
		Provider: provider,
		Model:    model,
		APIKey:   apiKey,
	}
	return config
}

// AsJSON converts the provider configuration to a JSON value for serialization.
// This method is used internally by the Compozy system for configuration persistence
// and inter-service communication.
//
// **Returns:** JSON representation of the configuration or an error if marshaling fails
func (p *ProviderConfig) AsJSON() (json.RawMessage, error) {
	return json.Marshal(p)
}

// AsMap converts the provider configuration to a map for template normalization.
// This method enables template processing and dynamic configuration resolution
// within the Compozy workflow system.
//
// **Returns:** Map representation suitable for template processing
func (p *ProviderConfig) AsMap() (map[string]any, error) {
	return AsMapDefault(p)
}

// FromMap updates the provider configuration from a normalized map.
// This method supports dynamic configuration updates and template resolution
// by merging new values with existing configuration.
//
// **Parameters:**
//   - data: Map containing configuration updates
//
// **Returns:** Error if the map cannot be converted or merged
func (p *ProviderConfig) FromMap(data any) error {
	config, err := FromMapDefault[ProviderConfig](data)
	if err != nil {
		return err
	}
	return mergo.Merge(p, config, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue)
}
