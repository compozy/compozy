package tokens

import (
	"context"
	"fmt"
	"os"
	"strings"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
)

// APIKeyResolver handles secure API key resolution from environment variables
type APIKeyResolver struct {
}

// NewAPIKeyResolver creates a new API key resolver
func NewAPIKeyResolver() *APIKeyResolver {
	return &APIKeyResolver{}
}

// ResolveAPIKey resolves the API key from the configuration
// Priority order:
// 1. If APIKeyEnv is set, use the environment variable it references
// 2. If APIKey starts with ${} pattern, treat it as an env var reference
// 3. Otherwise use APIKey directly (for development/testing only)
func (r *APIKeyResolver) ResolveAPIKey(ctx context.Context, config *memcore.TokenProviderConfig) string {
	log := logger.FromContext(ctx)
	if config.APIKeyEnv != "" {
		value := os.Getenv(config.APIKeyEnv)
		if value == "" {
			log.Warn("API key environment variable is not set",
				"provider", config.Provider)
		}
		return value
	}
	if strings.HasPrefix(config.APIKey, "${") && strings.HasSuffix(config.APIKey, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(config.APIKey, "${"), "}")
		value := os.Getenv(envVar)
		if value == "" {
			log.Warn("API key environment variable is not set",
				"provider", config.Provider)
		}
		return value
	}
	if config.APIKey != "" &&
		(os.Getenv("GO_ENV") == "production" || os.Getenv("APP_ENV") == "production") {
		log.Warn("API key is stored in plain text configuration - consider using environment variables",
			"provider", config.Provider)
	}
	return config.APIKey
}

// ResolveProviderConfig creates a new ProviderConfig with resolved API key
func (r *APIKeyResolver) ResolveProviderConfig(
	ctx context.Context,
	config *memcore.TokenProviderConfig,
) *ProviderConfig {
	return &ProviderConfig{
		Provider: config.Provider,
		Model:    config.Model,
		APIKey:   r.ResolveAPIKey(ctx, config),
		Endpoint: config.Endpoint,
		Settings: config.Settings,
	}
}

// GetRequiredEnvVars returns a list of required environment variables for a provider
func GetRequiredEnvVars(provider string) []string {
	switch strings.ToLower(provider) {
	case "openai":
		return []string{"OPENAI_API_KEY"}
	case "anthropic":
		return []string{"ANTHROPIC_API_KEY"}
	case "googleai", "google":
		return []string{"GOOGLE_API_KEY"}
	case "cohere":
		return []string{"COHERE_API_KEY"}
	case "deepseek":
		return []string{"DEEPSEEK_API_KEY"}
	default:
		return []string{fmt.Sprintf("%s_API_KEY", strings.ToUpper(provider))}
	}
}
