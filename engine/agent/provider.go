package agent

import (
	"context"
	"encoding/json"

	"github.com/compozy/compozy/pkg/ref"
	"github.com/pkg/errors"
)

// MessageRole represents the role of a message
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
	MessageRoleTool      MessageRole = "tool"
)

// Message represents a message configuration
type Message struct {
	Role    MessageRole `json:"role"    yaml:"role"`
	Content string      `json:"content" yaml:"content"`
}

// ProviderConfig represents provider-specific configuration options
type ProviderConfig struct {
	ref.WithRef
	Ref              *ref.Node    `json:"$ref,omitempty"              yaml:"$ref,omitempty"`
	Provider         ProviderName `json:"provider"                    yaml:"provider"`
	Model            ModelName    `json:"model"                       yaml:"model"`
	APIKey           string       `json:"api_key"                     yaml:"api_key"`
	APIURL           string       `json:"api_url"                     yaml:"api_url"`
	Temperature      float32      `json:"temperature,omitempty"       yaml:"temperature,omitempty"`
	MaxTokens        int32        `json:"max_tokens,omitempty"        yaml:"max_tokens,omitempty"`
	TopP             float32      `json:"top_p,omitempty"             yaml:"top_p,omitempty"`
	FrequencyPenalty float32      `json:"frequency_penalty,omitempty" yaml:"frequency_penalty,omitempty"`
	PresencePenalty  float32      `json:"presence_penalty,omitempty"  yaml:"presence_penalty,omitempty"`
}

// ResolveRef resolves all references within the provider configuration, including top-level $ref
func (p *ProviderConfig) ResolveRef(ctx context.Context, currentDoc map[string]any, projectRoot, filePath string) error {
	if p == nil {
		return nil
	}
	// Resolve provider $ref
	if p.Ref != nil && !p.Ref.IsEmpty() {
		p.SetRefMetadata(filePath, projectRoot)
		if err := p.WithRef.ResolveAndMergeNode(
			ctx,
			p.Ref,
			p,
			currentDoc,
			ref.ModeMerge,
		); err != nil {
			return errors.Wrap(err, "failed to resolve provider config reference")
		}
	}
	return nil
}

// AsJSON converts the provider configuration to a JSON value
func (p *Config) AsJSON() (json.RawMessage, error) {
	data, err := json.Marshal(p)
	return json.RawMessage(data), err
}

// NewProviderConfig creates a new ProviderConfig with the API URL populated
func NewProviderConfig(provider ProviderName, model ModelName, apiKey string) *ProviderConfig {
	config := &ProviderConfig{
		Provider: provider,
		Model:    model,
		APIKey:   apiKey,
	}
	// Populate APIURL using the Provider interface
	if p := GetProvider(provider); p != nil {
		config.APIURL = p.GetAPIURL()
	}
	return config
}
