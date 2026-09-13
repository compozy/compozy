package extensionpkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/marketplace"
)

// ManifestInput uses the same grammar as its published catalog entry.
type ManifestInput = marketplace.EntryInput

// Native TOML defaults must be decoded before crossing the JSON grammar boundary.
type manifestInputDocument struct {
	ID       string                   `toml:"id"                json:"id"`
	Prompt   string                   `toml:"prompt"            json:"prompt"`
	Type     string                   `toml:"type"              json:"type"`
	Required bool                     `toml:"required"          json:"required"`
	Binding  marketplace.InputBinding `toml:"binding"           json:"binding"`
	Default  any                      `toml:"default,omitempty" json:"default,omitempty"`
}

func decodeManifestInputs(documents []manifestInputDocument) ([]ManifestInput, error) {
	inputs := make([]ManifestInput, 0, len(documents))
	for index, document := range documents {
		input := ManifestInput{ID: strings.TrimSpace(document.ID), Prompt: strings.TrimSpace(document.Prompt),
			Type: document.Type, Required: document.Required, Binding: document.Binding}
		input.Binding.Name = strings.TrimSpace(input.Binding.Name)
		if document.Default != nil {
			value, err := json.Marshal(document.Default)
			if err != nil {
				return nil, &ManifestValidationError{
					Field:   fmt.Sprintf("inputs[%d].default", index),
					Message: err.Error(),
				}
			}
			input.Default = value
		}
		inputs = append(inputs, input)
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	return inputs, nil
}

func cloneManifestInputs(inputs []ManifestInput) []ManifestInput {
	if inputs == nil {
		return nil
	}
	cloned := make([]ManifestInput, len(inputs))
	for index, input := range inputs {
		cloned[index] = input
		cloned[index].Default = cloneManifestRawMessage(input.Default)
	}
	return cloned
}

func manifestRequiredEnv(existing []string, inputs []ManifestInput) []string {
	values := normalizeStrings(existing)
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, input := range inputs {
		if !input.Required || input.Binding.Type != "env" {
			continue
		}
		if _, exists := seen[input.Binding.Name]; exists {
			continue
		}
		seen[input.Binding.Name] = struct{}{}
		values = append(values, input.Binding.Name)
	}
	return values
}

// ValidateManifestInputs checks grammar and resolves every binding to declared servers.
func ValidateManifestInputs(manifest *Manifest) error {
	if manifest == nil {
		return nil
	}
	if err := marketplace.ValidateInputGrammar(manifest.Inputs); err != nil {
		field := "inputs"
		if grammarErr, ok := errors.AsType[*marketplace.InputGrammarError](err); ok {
			field = grammarErr.Field
		}
		return &ManifestValidationError{Field: field, Message: err.Error()}
	}
	for index, input := range manifest.Inputs {
		bound := false
		for _, name := range sortedMapKeys(manifest.Resources.MCPServers) {
			server := manifest.Resources.MCPServers[name]
			matches, err := manifestInputMatchesServer(input, server)
			if err != nil {
				return &ManifestValidationError{
					Field:   fmt.Sprintf("inputs[%d].binding", index),
					Message: fmt.Sprintf("server %q: %v", name, err),
				}
			}
			bound = bound || matches
		}
		if !bound {
			return &ManifestValidationError{
				Field:   fmt.Sprintf("inputs[%d].binding", index),
				Message: "input is not bound to any declared MCP server",
			}
		}
	}
	return nil
}

func manifestInputMatchesServer(input ManifestInput, server MCPServerConfig) (bool, error) {
	name := strings.TrimSpace(input.Binding.Name)
	if input.Binding.Type == "url_query" {
		parsed, err := url.Parse(server.URL)
		if err != nil {
			return false, fmt.Errorf("invalid URL: %w", err)
		}
		query, err := url.ParseQuery(parsed.RawQuery)
		if err != nil {
			return false, fmt.Errorf("invalid URL query: %w", err)
		}
		if !query.Has(name) {
			return false, nil
		}
		if server.Transport != "http" {
			return false, fmt.Errorf("url_query requires http transport")
		}
		return true, nil
	}
	value, plain := server.Env[name]
	secretValue, secret := server.SecretEnv[name]
	if !plain && !secret {
		return false, nil
	}
	if server.Transport != "" && server.Transport != "stdio" {
		return false, fmt.Errorf("env requires stdio transport")
	}
	if input.Type == "secret" {
		if plain || !secret || secretValue != input.ID {
			return false, fmt.Errorf("secret_env[%q] must name input %q", name, input.ID)
		}
	} else if secret || !plain || value != input.ID {
		return false, fmt.Errorf("env[%q] must name input %q", name, input.ID)
	}
	return true, nil
}

func (d *manifestInputDocument) UnmarshalJSON(data []byte) error {
	type document manifestInputDocument
	var decoded struct {
		*document
		Default json.RawMessage `json:"default"`
	}
	decoded.document = (*document)(d)
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if len(decoded.Default) > 0 {
		d.Default = decoded.Default
	}
	return nil
}

func manifestInputsTOML(inputs []ManifestInput) ([]manifestInputDocument, error) {
	documents := make([]manifestInputDocument, 0, len(inputs))
	for index, input := range inputs {
		document := manifestInputDocument{
			ID:       input.ID,
			Prompt:   input.Prompt,
			Type:     input.Type,
			Required: input.Required,
			Binding:  input.Binding,
		}
		if len(input.Default) > 0 {
			if err := json.Unmarshal(input.Default, &document.Default); err != nil {
				return nil, fmt.Errorf("inputs[%d].default: %w", index, err)
			}
		}
		documents = append(documents, document)
	}
	return documents, nil
}
