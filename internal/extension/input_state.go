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
	// Invalid names why a stored value fails the current manifest type; the value is never kept.
	Invalid string
}

// InputState is the effective input snapshot for one extension instance.
type InputState struct{ Values map[string]InputValueRecord }

// Readiness separates missing typed inputs from requires_env requirements.
type Readiness struct {
	MissingInputs []string
	MissingEnv    []string
	// InvalidInputs have a stored value that fails the current manifest type; required ones also block.
	InvalidInputs []string
	// DaemonEnvInputs are server-and-subprocess inputs satisfied only by the daemon environment.
	DaemonEnvInputs []string
}

// RequiredError names manifest input IDs while retaining the separate missing environment list.
func (r Readiness) RequiredError(manifest *Manifest) error {
	if len(r.MissingInputs) == 0 && len(r.MissingEnv) == 0 {
		return nil
	}
	missing := r.missingInputIDs(manifest)
	var definitions []ManifestInput
	if manifest != nil {
		for _, input := range manifest.Inputs {
			if slices.Contains(missing, input.ID) {
				input.Default = slices.Clone(input.Default)
				definitions = append(definitions, input)
			}
		}
	}
	var invalid []string
	for _, id := range r.InvalidInputs {
		if slices.Contains(missing, id) {
			invalid = append(invalid, id)
		}
	}
	return &InputsRequiredError{
		MissingInputs: missing, MissingEnv: slices.Clone(r.MissingEnv), InputDefinitions: definitions,
		InvalidInputs: invalid,
	}
}

// missingInputIDs names every blocking input: typed missing inputs plus env-bound inputs whose
// requires_env name is missing or invalid.
func (r Readiness) missingInputIDs(manifest *Manifest) []string {
	missing := append([]string{}, r.MissingInputs...)
	if manifest != nil {
		for _, input := range manifest.Inputs {
			if input.Binding.Type == manifestEnvKey && slices.Contains(r.MissingEnv, input.Binding.Name) {
				missing = append(missing, input.ID)
			}
		}
	}
	slices.Sort(missing)
	return slices.Compact(missing)
}

var ErrExtensionInputInvalid = errors.New("extension: invalid input")
var ErrExtensionInputsRequired = errors.New("extension: inputs required")
var ErrExtensionInputOverwriteDenied = errors.New("extension: input overwrite denied")

// InputOverwriteDeniedError refuses an agent write that targets inputs already set (D7); nothing is written.
type InputOverwriteDeniedError struct{ InputIDs []string }

func (e *InputOverwriteDeniedError) Error() string {
	return fmt.Sprintf(
		"extension: inputs already set; only the operator can overwrite them: %s",
		strings.Join(e.InputIDs, ", "),
	)
}
func (e *InputOverwriteDeniedError) Unwrap() error { return ErrExtensionInputOverwriteDenied }

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
	InputDefinitions []ManifestInput
	MissingInputs    []string
	MissingEnv       []string
	// InvalidInputs lists the required inputs among MissingInputs whose stored value fails the manifest type.
	InvalidInputs []string
	// Digest is the SHA-256 of the acquired package an install refusal describes, so a retry with the
	// prompted values can pin the same bytes.
	Digest string
}

func (e *InputsRequiredError) Unwrap() error { return ErrExtensionInputsRequired }

func (e *InputsRequiredError) Error() string {
	return fmt.Sprintf(
		"extension: inputs required: %s; environment required: %s",
		strings.Join(e.MissingInputs, ", "),
		strings.Join(e.MissingEnv, ", "),
	)
}

// InputReadiness uses stored values/defaults for URL inputs and permits environment fallback only for
// env inputs that an MCP server consumes; subprocess-only inputs need a binding or a default.
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
		result.addInput(manifest, input, state, getenv, satisfiedEnv)
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
	slices.Sort(result.InvalidInputs)
	slices.Sort(result.DaemonEnvInputs)
	return result
}

func (r *Readiness) addInput(
	manifest *Manifest, input ManifestInput, state InputState, getenv func(string) string,
	satisfiedEnv map[string]bool,
) {
	_, _, present, err := manifestInputValue(manifest, input, state, getenv)
	if err != nil {
		r.InvalidInputs = append(r.InvalidInputs, input.ID)
	} else if present && manifest.daemonEnvInput(input, state, getenv) {
		r.DaemonEnvInputs = append(r.DaemonEnvInputs, input.ID)
	}
	usable := present && err == nil
	if input.Binding.Type != manifestEnvKey {
		if input.Required && !usable {
			r.MissingInputs = append(r.MissingInputs, input.ID)
		}
		return
	}
	// An optional subprocess input never blocks start through its requires_env key.
	satisfied := usable || (!input.Required && manifest.SubprocessInput(input))
	satisfiedEnv[input.Binding.Name] = satisfiedEnv[input.Binding.Name] || satisfied
	if input.Required && !usable && !slices.Contains(r.MissingEnv, input.Binding.Name) {
		r.MissingEnv = append(r.MissingEnv, input.Binding.Name)
	}
}

// manifestInputValue withholds the daemon environment from subprocess-only inputs (rule 5c).
func manifestInputValue(
	manifest *Manifest, input ManifestInput, state InputState, getenv func(string) string,
) (value, ref string, present bool, err error) {
	if manifest.SubprocessOnlyInput(input) {
		getenv = func(string) string { return "" }
	}
	return effectiveInputValue(input, state, getenv)
}

// daemonEnvInput reports a server-and-subprocess input with no stored value or default whose only
// source is the daemon environment, which the subprocess never receives (rule 5a/5d). A released
// manifest's required server env input is folded into RequiresEnv without an author declaration and
// reaches the subprocess only through bindings, so it qualifies too.
func (m *Manifest) daemonEnvInput(input ManifestInput, state InputState, getenv func(string) string) bool {
	if input.Binding.Type != manifestEnvKey || !slices.Contains(m.RequiresEnv, strings.TrimSpace(input.Binding.Name)) ||
		!m.ServerInput(input) || len(input.Default) > 0 {
		return false
	}
	if record, found := state.Values[input.ID]; found && record.Active {
		return false
	}
	return strings.TrimSpace(getenv(input.Binding.Name)) != ""
}

func effectiveInputValue(
	input ManifestInput,
	state InputState,
	getenv func(string) string,
) (value, ref string, present bool, err error) {
	if record, found := state.Values[input.ID]; found && record.Active {
		if record.Invalid != "" {
			return "", "", false, &InputValidationError{input.ID, record.Invalid}
		}
		if record.Type != input.Type {
			return "", "", false, &InputValidationError{input.ID, "stored type does not match the manifest"}
		}
		if input.Type == manifestInputTypeSecret {
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
	// A secret has no default (rule 2) and is checked for presence only, so a stray default is ignored.
	if len(input.Default) > 0 && input.Type != manifestInputTypeSecret {
		value, err := typedInputValue(input.Type, input.Default)
		if err != nil {
			return "", "", false, &InputValidationError{input.ID, err.Error()}
		}
		return value, "", true, nil
	}
	if input.Binding.Type == manifestEnvKey {
		if getenv == nil {
			getenv = os.Getenv
		}
		raw := getenv(input.Binding.Name)
		if strings.TrimSpace(raw) != "" {
			if input.Type == manifestInputTypeSecret {
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
	if inputType == manifestInputTypeBoolean {
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
	if inputType == manifestInputTypeSecret {
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
	if inputType == manifestInputTypeBoolean {
		return json.RawMessage(value), nil
	}
	return json.Marshal(value)
}
