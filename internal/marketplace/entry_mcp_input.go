package marketplace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	mcpInputTypeString     = "string"
	mcpInputTypeIdentifier = "identifier"
	mcpInputTypeBoolean    = "boolean"
	mcpInputTypeSecret     = "secret"
	mcpInputBindingEnv     = "env"
	mcpInputBindingQuery   = "url_query"
	maxMCPInputValueBytes  = 8 * 1024
)

type mcpInput struct {
	ID       string          `json:"id"`
	Prompt   string          `json:"prompt"`
	Type     string          `json:"type"`
	Required bool            `json:"required"`
	Binding  mcpInputBinding `json:"binding"`
	Default  json.RawMessage `json:"default,omitempty"`
}

type mcpInputBinding struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func validateMCPInputs(entryID string, launch mcpLaunch, inputs []mcpInput) error {
	seen := make(map[string]struct{}, len(inputs))
	seenBindings := make(map[string]struct{}, len(inputs))
	urlQueryNames, err := launchURLQueryNames(launch)
	if err != nil {
		return fmt.Errorf("marketplace catalog MCP entry %q: %w", entryID, err)
	}
	for index, input := range inputs {
		if err := input.validate(entryID, launch.Type, index); err != nil {
			return err
		}
		id := strings.TrimSpace(input.ID)
		if _, exists := seen[id]; exists {
			return fmt.Errorf("marketplace catalog MCP entry %q input id %q is duplicated", entryID, id)
		}
		seen[id] = struct{}{}
		binding := input.Binding.Type + "\x00" + strings.TrimSpace(input.Binding.Name)
		if _, exists := seenBindings[binding]; exists {
			return fmt.Errorf(
				"marketplace catalog MCP entry %q input binding %s/%q is duplicated",
				entryID,
				input.Binding.Type,
				strings.TrimSpace(input.Binding.Name),
			)
		}
		seenBindings[binding] = struct{}{}
		if input.Binding.Type == mcpInputBindingQuery {
			if _, exists := urlQueryNames[strings.TrimSpace(input.Binding.Name)]; exists {
				return fmt.Errorf(
					"marketplace catalog MCP entry %q input binding url_query/%q conflicts with launch URL",
					entryID,
					strings.TrimSpace(input.Binding.Name),
				)
			}
		}
	}
	return nil
}

func launchURLQueryNames(launch mcpLaunch) (map[string]struct{}, error) {
	names := make(map[string]struct{})
	if launch.Type != mcpLaunchTypeRemote {
		return names, nil
	}
	parsed, err := url.Parse(launch.URL)
	if err != nil {
		return nil, fmt.Errorf("parse launch URL: %w", err)
	}
	for name := range parsed.Query() {
		names[name] = struct{}{}
	}
	return names, nil
}

func (i mcpInput) validate(entryID string, launchType string, index int) error {
	prefix := fmt.Sprintf("marketplace catalog MCP entry %q inputs[%d]", entryID, index)
	if id := strings.TrimSpace(i.ID); id == "" || !entryIDPattern.MatchString(id) {
		return fmt.Errorf("%s.id must be one URL-safe path segment", prefix)
	}
	if strings.TrimSpace(i.Prompt) == "" {
		return fmt.Errorf("%s.prompt is required", prefix)
	}
	if err := i.validateBinding(prefix, launchType); err != nil {
		return err
	}
	return i.validateDefault(prefix)
}

func (i mcpInput) validateBinding(prefix string, launchType string) error {
	name := strings.TrimSpace(i.Binding.Name)
	switch i.Type {
	case mcpInputTypeString, mcpInputTypeIdentifier, mcpInputTypeBoolean, mcpInputTypeSecret:
	default:
		return fmt.Errorf("%s.type must be string, identifier, boolean, or secret", prefix)
	}
	switch i.Binding.Type {
	case mcpInputBindingEnv:
		if launchType == mcpLaunchTypeRemote {
			return fmt.Errorf("%s.binding env is only supported for stdio launch", prefix)
		}
		if err := compozyconfig.ValidateMCPStdioEnvName(
			prefix+".binding",
			name,
			i.Type == mcpInputTypeSecret,
		); err != nil {
			return err
		}
	case mcpInputBindingQuery:
		if launchType != mcpLaunchTypeRemote {
			return fmt.Errorf("%s.binding url_query is only supported for remote launch", prefix)
		}
		if i.Type == mcpInputTypeSecret {
			return fmt.Errorf("%s.binding url_query must not bind a secret", prefix)
		}
		if name == "" || !entryIDPattern.MatchString(name) {
			return fmt.Errorf("%s.binding.name must be one URL-safe query parameter", prefix)
		}
	default:
		return fmt.Errorf("%s.binding.type must be env or url_query", prefix)
	}
	return nil
}

func (i mcpInput) validateDefault(prefix string) error {
	if len(i.Default) == 0 {
		return nil
	}
	if i.Type == mcpInputTypeSecret {
		return fmt.Errorf("%s.default must not be set for secret inputs", prefix)
	}
	decoder := json.NewDecoder(bytes.NewReader(i.Default))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("%s.default must be valid JSON: %w", prefix, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%s.default must not have trailing JSON", prefix)
		}
		return fmt.Errorf("%s.default has trailing JSON: %w", prefix, err)
	}
	switch i.Type {
	case mcpInputTypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s.default must be a boolean", prefix)
		}
	case mcpInputTypeString, mcpInputTypeIdentifier:
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s.default must be a string for %s inputs", prefix, i.Type)
		}
		if _, err := NormalizeMCPInputValue(i.Type, text); err != nil {
			return fmt.Errorf("%s.default is invalid: %w", prefix, err)
		}
	}
	return nil
}

// NormalizeMCPInputValue validates and canonicalizes a catalog input value.
func NormalizeMCPInputValue(inputType string, raw string) (string, error) {
	if err := ValidateMCPInputValue(raw); err != nil {
		return "", err
	}
	switch inputType {
	case mcpInputTypeString:
		return raw, nil
	case mcpInputTypeIdentifier:
		value := strings.TrimSpace(raw)
		if value == "" {
			return "", errors.New("identifier must not be blank")
		}
		for _, runeValue := range value {
			isUnreserved := (runeValue >= 'a' && runeValue <= 'z') ||
				(runeValue >= 'A' && runeValue <= 'Z') ||
				(runeValue >= '0' && runeValue <= '9') ||
				runeValue == '.' || runeValue == '_' || runeValue == '~' || runeValue == '-'
			if !isUnreserved {
				return "", errors.New("identifier must use URL-safe unreserved characters")
			}
		}
		return value, nil
	case mcpInputTypeBoolean:
		value, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return "", errors.New("boolean must be true or false")
		}
		return strconv.FormatBool(value), nil
	default:
		return "", fmt.Errorf("unsupported input type %q", inputType)
	}
}

// ValidateMCPInputValue enforces transport-safe bounds shared by feed defaults and install values.
func ValidateMCPInputValue(value string) error {
	if strings.ContainsRune(value, '\x00') {
		return errors.New("value must not contain NUL")
	}
	if len(value) > maxMCPInputValueBytes {
		return fmt.Errorf("value exceeds %d bytes", maxMCPInputValueBytes)
	}
	return nil
}
