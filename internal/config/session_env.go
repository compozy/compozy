package config

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/compozy/compozy/internal/vault"
	"github.com/goccy/go-yaml"
)

const (
	sessionEnvRefField      = "ref"
	sessionEnvRequiredField = "required"
)

// SessionEnvBinding declares a launch-time reference, never a resolved value.
type SessionEnvBinding struct {
	Ref      string `json:"ref"      yaml:"ref"      toml:"ref"`
	Required bool   `json:"required" yaml:"required" toml:"required"`
}

var _ toml.Unmarshaler = (*SessionEnvBinding)(nil)
var _ yaml.BytesUnmarshaler = (*SessionEnvBinding)(nil)

func (b *SessionEnvBinding) UnmarshalTOML(value any) error { return b.decode(value) }

func (b *SessionEnvBinding) UnmarshalYAML(data []byte) error {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return errors.New("session_env: invalid declaration")
	}
	return b.decode(value)
}

func (b *SessionEnvBinding) UnmarshalJSON(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("session_env: invalid declaration")
	}
	return b.decode(value)
}

func (b *SessionEnvBinding) decode(value any) error {
	var binding SessionEnvBinding
	switch value := value.(type) {
	case string:
		binding.Ref = value
		binding.Required = true
	case map[string]any:
		ref, ok := value[sessionEnvRefField].(string)
		if !ok {
			return errors.New("session_env: ref must be a string")
		}
		binding.Ref = ref
		binding.Required = true
		for key, field := range value {
			switch key {
			case sessionEnvRefField:
			case sessionEnvRequiredField:
				required, ok := field.(bool)
				if !ok {
					return errors.New("session_env: required must be a boolean")
				}
				binding.Required = required
			default:
				return errors.New("session_env: only ref and required fields are allowed")
			}
		}
	default:
		return errors.New("session_env: expected an env:/vault: reference or a ref/required table")
	}
	binding.Ref = vault.NormalizeRef(binding.Ref)
	if err := binding.Validate(); err != nil {
		return err
	}
	*b = binding
	return nil
}

func (b SessionEnvBinding) Validate() error {
	if (!vault.IsSecretRef(b.Ref) && !vault.IsEnvRef(b.Ref)) || vault.ValidateRef(b.Ref) != nil {
		return errors.New("session_env: ref must be a valid env:NAME or vault:path reference")
	}
	return nil
}

// SessionEnv contains only explicitly declared environment names.
type SessionEnv map[string]SessionEnvBinding

func (env SessionEnv) Validate(path string) error {
	for _, name := range slices.Sorted(maps.Keys(env)) {
		if !vault.EnvNamePattern.MatchString(name) {
			return fmt.Errorf("%s: invalid environment variable name", path)
		}
		if strings.HasPrefix(strings.ToUpper(name), "COMPOZY_") {
			return fmt.Errorf("%s: COMPOZY_ environment names are reserved", path)
		}
		if err := env[name].Validate(); err != nil {
			return fmt.Errorf("%s.%s: %w", path, name, err)
		}
	}
	return nil
}

// MergeSessionEnv applies declarations by name without sharing mutable maps.
func MergeSessionEnv(base, override SessionEnv) SessionEnv {
	if len(base)+len(override) == 0 {
		return nil
	}
	result := make(SessionEnv, len(base)+len(override))
	maps.Copy(result, base)
	maps.Copy(result, override)
	return result
}

// LaunchFingerprint binds native session reuse to the command and declared references.
// Routes without declarations retain their persisted pre-session_env identity.
func (agent ResolvedAgent) LaunchFingerprint() string {
	if len(agent.SessionEnv) == 0 {
		return CommandFingerprint(agent.Command)
	}
	var identity strings.Builder
	identity.WriteString("session_env:v1:")
	identity.WriteString(strconv.Quote(agent.Command))
	for _, name := range slices.Sorted(maps.Keys(agent.SessionEnv)) {
		binding := agent.SessionEnv[name]
		identity.WriteString(strconv.Quote(name))
		identity.WriteString(strconv.Quote(binding.Ref))
		identity.WriteString(strconv.FormatBool(binding.Required))
	}
	return CommandFingerprint(identity.String())
}
