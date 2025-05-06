package provider

// ProviderName represents the name of a provider
type ProviderName string

const (
	ProviderOpenAI     ProviderName = "openai"
	ProviderGroq       ProviderName = "groq"
	ProviderAnthropic  ProviderName = "anthropic"
	ProviderMistral    ProviderName = "mistral"
	ProviderCohere     ProviderName = "cohere"
	ProviderPerplexity ProviderName = "perplexity"
	ProviderXAI        ProviderName = "xai"
	ProviderGoogle     ProviderName = "google"
)

// ModelName represents the name of a model
type ModelName string

const (
	// OpenAI models
	ModelGPT4o     ModelName = "gpt-4o"
	ModelGPT4oMini ModelName = "gpt-4o-mini"
	ModelGPT41     ModelName = "gpt-4.1"
	ModelGPT41Mini ModelName = "gpt-4.1-mini"
	ModelGPT41Nano ModelName = "gpt-4.1-nano"
	ModelGPT45     ModelName = "gpt-4.5"
	ModelO1        ModelName = "o1"
	ModelO3        ModelName = "o3"
	ModelO3Mini    ModelName = "o3-mini"
	ModelO4Mini    ModelName = "o4-mini"

	// Groq models (also available via OpenRouter)
	ModelLLama270b           ModelName = "llama2-70b-4096"
	ModelLLama213b           ModelName = "llama2-13b-4096"
	ModelMixtral8x7b         ModelName = "mixtral-8x7b-32768"
	ModelGemma7b             ModelName = "gemma-7b-it"
	ModelLLama3370bVersatile ModelName = "llama-3.3-70b-versatile"
	ModelLLama4Maverick17b   ModelName = "llama-4-maverick-17b-instruct"
	ModelLLama4Scout17b      ModelName = "llama-4-scout-17b-instruct"

	// Anthropic models
	ModelClaude3Opus   ModelName = "claude-3-opus-20240229"
	ModelClaude3Sonnet ModelName = "claude-3-sonnet-20240229"
	ModelClaude3Haiku  ModelName = "claude-3-haiku-20240307"

	// Mistral models
	ModelMistralLarge  ModelName = "mistral-large-latest"
	ModelMistralMedium ModelName = "mistral-medium-latest"
	ModelMistralSmall  ModelName = "mistral-small-latest"

	// Cohere models
	ModelCommand        ModelName = "command"
	ModelCommandLight   ModelName = "command-light"
	ModelCommandNightly ModelName = "command-nightly"

	// Perplexity models
	ModelPPLX7BOnline  ModelName = "pplx-7b-online"
	ModelPPLX70BOnline ModelName = "pplx-70b-online"
	ModelPPLX7B        ModelName = "pplx-7b"
	ModelPPLX70B       ModelName = "pplx-70b"

	// xAI models (available via OpenRouter)
	ModelGrok3     ModelName = "grok-3"
	ModelGrok3Mini ModelName = "grok-3-mini"

	// Google models
	ModelGemini25Pro   ModelName = "gemini-2.5-pro"
	ModelGemini20Flash ModelName = "gemini-2.0-flash"
	ModelGemini25Flash ModelName = "gemini-2.5-flash"
	ModelGemma3        ModelName = "gemma-3"
)

// Provider defines an interface for retrieving the API URL of a provider
type Provider interface {
	GetAPIURL() string
}

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct{}

func (p *OpenAIProvider) GetAPIURL() string {
	return "https://api.openai.com/v1"
}

// GroqProvider implements the Provider interface for Groq
type GroqProvider struct{}

func (p *GroqProvider) GetAPIURL() string {
	return "https://api.groq.com/openai/v1"
}

// AnthropicProvider implements the Provider interface for Anthropic
type AnthropicProvider struct{}

func (p *AnthropicProvider) GetAPIURL() string {
	return "https://api.anthropic.com/v1"
}

// MistralProvider implements the Provider interface for Mistral
type MistralProvider struct{}

func (p *MistralProvider) GetAPIURL() string {
	return "https://api.mixtral.ai/v1"
}

// CohereProvider implements the Provider interface for Cohere
type CohereProvider struct{}

func (p *CohereProvider) GetAPIURL() string {
	return "https://api.cohere.ai/v1"
}

// PerplexityProvider implements the Provider interface for Perplexity
type PerplexityProvider struct{}

func (p *PerplexityProvider) GetAPIURL() string {
	return "https://api.perplexity.ai/v1"
}

// XAIProvider implements the Provider interface for xAI
type XAIProvider struct{}

func (p *XAIProvider) GetAPIURL() string {
	return "https://api.x.ai/v1"
}

// GoogleProvider implements the Provider interface for Google
type GoogleProvider struct{}

func (p *GoogleProvider) GetAPIURL() string {
	return "https://generativelanguage.googleapis.com/v1"
}

// GetProvider returns a Provider instance based on the ProviderName
func GetProvider(name ProviderName) Provider {
	switch name {
	case ProviderOpenAI:
		return &OpenAIProvider{}
	case ProviderGroq:
		return &GroqProvider{}
	case ProviderAnthropic:
		return &AnthropicProvider{}
	case ProviderMistral:
		return &MistralProvider{}
	case ProviderCohere:
		return &CohereProvider{}
	case ProviderPerplexity:
		return &PerplexityProvider{}
	case ProviderXAI:
		return &XAIProvider{}
	case ProviderGoogle:
		return &GoogleProvider{}
	default:
		return nil // or handle unknown provider case as needed
	}
}
