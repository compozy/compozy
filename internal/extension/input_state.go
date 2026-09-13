package extensionpkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/marketplace"
)

// InputValueRecord holds a typed persisted value or a secret reference, never secret plaintext.
type InputValueRecord struct {
	Type      string
	Value     json.RawMessage
	SecretRef string
	Active    bool
	UpdatedAt time.Time
}

// InputState is the effective input snapshot for one extension instance.
type InputState struct{ Values map[string]InputValueRecord }

// Readiness separates missing typed inputs from legacy environment requirements.
type Readiness struct {
	MissingInputs []string
	MissingEnv    []string
}

// RequiredError names manifest input IDs while retaining the separate missing environment list.
func (r Readiness) RequiredError(manifest *Manifest) error {
	if len(r.MissingInputs) == 0 && len(r.MissingEnv) == 0 {
		return nil
	}
	missing := slices.Clone(r.MissingInputs)
	if manifest != nil {
		for _, input := range manifest.Inputs {
			if input.Binding.Type == "env" && slices.Contains(r.MissingEnv, input.Binding.Name) {
				missing = append(missing, input.ID)
			}
		}
	}
	slices.Sort(missing)
	return &InputsRequiredError{MissingInputs: missing, MissingEnv: slices.Clone(r.MissingEnv)}
}

var ErrExtensionInputInvalid = errors.New("extension: invalid input")
var ErrExtensionInputsRequired = errors.New("extension: inputs required")

// InputValidationError identifies invalid input without including its value.
type InputValidationError struct {
	InputID string
	Reason  string
}

func (e *InputValidationError) Error() string {
	return fmt.Sprintf("extension: input %q: %s", e.InputID, e.Reason)
}
func (e *InputValidationError) Unwrap() error { return ErrExtensionInputInvalid }

// InputsRequiredError reports the information required before an extension can run.
type InputsRequiredError struct {
	MissingInputs []string
	MissingEnv    []string
}

func (e *InputsRequiredError) Unwrap() error { return ErrExtensionInputsRequired }

func (e *InputsRequiredError) Error() string {
	return fmt.Sprintf(
		"extension: inputs required: %s; environment required: %s",
		strings.Join(e.MissingInputs, ", "),
		strings.Join(e.MissingEnv, ", "),
	)
}

// InputReadiness uses stored values/defaults for URL inputs and permits environment fallback only for env inputs.
func InputReadiness(manifest *Manifest, state InputState, getenv func(string) string) Readiness {
	result := Readiness{MissingInputs: []string{}, MissingEnv: []string{}}
	if manifest == nil {
		return result
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	satisfiedEnv := make(map[string]bool)
	for _, input := range manifest.Inputs {
		_, _, present, err := effectiveInputValue(input, state, getenv)
		if input.Binding.Type == "env" {
			satisfiedEnv[input.Binding.Name] = present && err == nil
			if input.Required && !satisfiedEnv[input.Binding.Name] {
				result.MissingEnv = append(result.MissingEnv, input.Binding.Name)
			}
		} else if input.Required && (!present || err != nil) {
			result.MissingInputs = append(result.MissingInputs, input.ID)
		}
	}
	for _, name := range manifest.RequiresEnv {
		present, declared := satisfiedEnv[name]
		if !declared {
			present = strings.TrimSpace(getenv(name)) != ""
		}
		if !present && !slices.Contains(result.MissingEnv, name) {
			result.MissingEnv = append(result.MissingEnv, name)
		}
	}
	slices.Sort(result.MissingEnv)
	slices.Sort(result.MissingInputs)
	return result
}

func effectiveInputValue(
	input ManifestInput,
	state InputState,
	getenv func(string) string,
) (value, ref string, present bool, err error) {
	if record, found := state.Values[input.ID]; found && record.Active {
		if record.Type != input.Type {
			return "", "", false, &InputValidationError{input.ID, "stored type does not match the manifest"}
		}
		if input.Type == "secret" {
			if strings.TrimSpace(record.SecretRef) == "" {
				return "", "", false, &InputValidationError{input.ID, "stored secret reference is empty"}
			}
			return "", record.SecretRef, true, nil
		}
		value, err := typedInputValue(input.Type, record.Value)
		if err != nil {
			return "", "", false, &InputValidationError{input.ID, err.Error()}
		}
		return value, "", true, nil
	}
	if len(input.Default) > 0 {
		value, err := typedInputValue(input.Type, input.Default)
		if err != nil {
			return "", "", false, &InputValidationError{input.ID, err.Error()}
		}
		return value, "", true, nil
	}
	if input.Binding.Type == "env" {
		if getenv == nil {
			getenv = os.Getenv
		}
		raw := getenv(input.Binding.Name)
		if strings.TrimSpace(raw) != "" {
			if input.Type == "secret" {
				return "", "env:" + input.Binding.Name, true, nil
			}
			value, err := marketplace.NormalizeMCPInputValue(input.Type, raw)
			if err != nil {
				return "", "", false, &InputValidationError{input.ID, err.Error()}
			}
			return value, "", true, nil
		}
	}
	return "", "", false, nil
}

func typedInputValue(inputType string, raw json.RawMessage) (string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", errors.New("value must be valid JSON")
	}
	if inputType == "boolean" {
		flag, ok := value.(bool)
		if !ok {
			return "", errors.New("value must be a boolean")
		}
		return strconv.FormatBool(flag), nil
	}
	if inputType != "string" && inputType != "identifier" {
		return "", errors.New("value must use a non-secret input type")
	}
	text, ok := value.(string)
	if !ok {
		return "", errors.New("value must be a string")
	}
	return marketplace.NormalizeMCPInputValue(inputType, text)
}

// NormalizeInputJSON validates a supplied value and keeps booleans as JSON booleans.
func NormalizeInputJSON(inputType string, raw json.RawMessage) (json.RawMessage, error) {
	if inputType == "secret" {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
			return nil, errors.New("secret must be a non-empty string")
		}
		if err := marketplace.ValidateMCPInputValue(value); err != nil {
			return nil, err
		}
		return json.Marshal(value)
	}
	value, err := typedInputValue(inputType, raw)
	if err != nil {
		return nil, err
	}
	if inputType == "boolean" {
		return json.RawMessage(value), nil
	}
	return json.Marshal(value)
}
