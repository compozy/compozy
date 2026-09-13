package marketplace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// EntryInput declares a typed value without depending on a server transport.
type EntryInput struct {
	ID       string          `json:"id"`
	Prompt   string          `json:"prompt"`
	Type     string          `json:"type"              toml:"type"`
	Required bool            `json:"required"`
	Binding  InputBinding    `json:"binding"`
	Default  json.RawMessage `json:"default,omitempty"`
}

// InputBinding names the environment variable or URL parameter receiving an input.
type InputBinding struct {
	Type string `toml:"type" json:"type"`
	Name string `toml:"name" json:"name"`
}

func (i EntryInput) validateDefault(prefix string) error {
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
	case mcpInputTypeString, mcpInputTypeSecret:
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

// InputGrammarError retains the input position for authoring diagnostics.
type InputGrammarError struct {
	Field string
	Err   error
}

func (e *InputGrammarError) Error() string { return e.Err.Error() }
func (e *InputGrammarError) Unwrap() error { return e.Err }

// ValidateInputGrammar validates inputs independently of the servers that consume them.
func ValidateInputGrammar(inputs []EntryInput) error {
	ids := make(map[string]struct{}, len(inputs))
	bindings := make(map[string]struct{}, len(inputs))
	for index, input := range inputs {
		prefix := fmt.Sprintf("inputs[%d]", index)
		if err := input.validateGrammar(prefix); err != nil {
			return &InputGrammarError{Field: prefix, Err: err}
		}
		id := strings.TrimSpace(input.ID)
		if _, exists := ids[id]; exists {
			return &InputGrammarError{Field: prefix + ".id", Err: fmt.Errorf("%s.id %q is duplicated", prefix, id)}
		}
		ids[id] = struct{}{}
		binding := input.Binding.Type + "\x00" + strings.TrimSpace(input.Binding.Name)
		if _, exists := bindings[binding]; exists {
			return &InputGrammarError{
				Field: prefix + ".binding",
				Err:   fmt.Errorf("%s.binding %s/%q is duplicated", prefix, input.Binding.Type, input.Binding.Name),
			}
		}
		bindings[binding] = struct{}{}
	}
	return nil
}

func (i EntryInput) validateGrammar(prefix string) error {
	if id := strings.TrimSpace(i.ID); id == "" || !entryIDPattern.MatchString(id) {
		return fmt.Errorf("%s.id must be one URL-safe path segment", prefix)
	}
	if strings.TrimSpace(i.Prompt) == "" {
		return fmt.Errorf("%s.prompt is required", prefix)
	}
	switch i.Type {
	case mcpInputTypeString, mcpInputTypeIdentifier, mcpInputTypeBoolean, mcpInputTypeSecret:
	default:
		return fmt.Errorf("%s.type must be string, identifier, boolean, or secret", prefix)
	}
	name := strings.TrimSpace(i.Binding.Name)
	switch i.Binding.Type {
	case mcpInputBindingEnv:
		if err := compozyconfig.ValidateMCPStdioEnvName(
			prefix+".binding",
			name,
			i.Type == mcpInputTypeSecret,
		); err != nil {
			return err
		}
	case mcpInputBindingQuery:
		if i.Type == mcpInputTypeSecret {
			return fmt.Errorf("%s.binding url_query must not bind a secret", prefix)
		}
		if name == "" || !entryIDPattern.MatchString(name) {
			return fmt.Errorf("%s.binding.name must be one URL-safe query parameter", prefix)
		}
	default:
		return fmt.Errorf("%s.binding.type must be env or url_query", prefix)
	}
	return i.validateDefault(prefix)
}
