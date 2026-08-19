package dsl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// StopWhenSpec accepts a plain expression or an object with an error-policy override.
type StopWhenSpec struct {
	Expr        string          `json:"expr"                    yaml:"expr"`
	OnEvalError EvalErrorPolicy `json:"on_eval_error,omitempty" yaml:"on_eval_error,omitempty"`
}

// IsZero lets encoding callers omit an unauthored stop condition.
func (s StopWhenSpec) IsZero() bool {
	return strings.TrimSpace(s.Expr) == "" && s.OnEvalError == ""
}

// UnmarshalYAML decodes the strict string-or-object authoring form.
func (s *StopWhenSpec) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return fmt.Errorf("stop_when is required")
	}
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Tag != "!!str" {
			return fmt.Errorf("stop_when must be a string or object")
		}
		*s = StopWhenSpec{Expr: strings.TrimSpace(value.Value)}
		return s.validate()
	case yaml.MappingNode:
		var decoded struct {
			Expr        string          `yaml:"expr"`
			OnEvalError EvalErrorPolicy `yaml:"on_eval_error,omitempty"`
		}
		if err := value.Decode(&decoded); err != nil {
			return fmt.Errorf("decode stop_when: %w", err)
		}
		if err := rejectUnknownStopWhenYAMLFields(value); err != nil {
			return err
		}
		*s = StopWhenSpec{Expr: strings.TrimSpace(decoded.Expr), OnEvalError: decoded.OnEvalError}
		return s.validate()
	default:
		return fmt.Errorf("stop_when must be a string or object")
	}
}

// MarshalYAML preserves the compact scalar form when no override was authored.
func (s StopWhenSpec) MarshalYAML() (any, error) {
	if err := s.validateOptional(); err != nil {
		return nil, err
	}
	if s.OnEvalError == "" {
		return s.Expr, nil
	}
	return struct {
		Expr        string          `yaml:"expr"`
		OnEvalError EvalErrorPolicy `yaml:"on_eval_error"`
	}{Expr: s.Expr, OnEvalError: s.OnEvalError}, nil
}

// UnmarshalJSON decodes the same strict forms at JSON ingress.
func (s *StopWhenSpec) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("stop_when must be a string or object")
	}
	if trimmed[0] == '"' {
		var expr string
		if err := json.Unmarshal(trimmed, &expr); err != nil {
			return fmt.Errorf("decode stop_when string: %w", err)
		}
		*s = StopWhenSpec{Expr: strings.TrimSpace(expr)}
		return s.validate()
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	var decoded struct {
		Expr        string          `json:"expr"`
		OnEvalError EvalErrorPolicy `json:"on_eval_error,omitempty"`
	}
	if err := decoder.Decode(&decoded); err != nil {
		return fmt.Errorf("decode stop_when object: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode stop_when object: trailing value")
		}
		return fmt.Errorf("decode stop_when object trailing value: %w", err)
	}
	*s = StopWhenSpec{Expr: strings.TrimSpace(decoded.Expr), OnEvalError: decoded.OnEvalError}
	return s.validate()
}

// MarshalJSON preserves scalar JSON for the default policy.
func (s StopWhenSpec) MarshalJSON() ([]byte, error) {
	if err := s.validateOptional(); err != nil {
		return nil, err
	}
	if s.OnEvalError == "" {
		return json.Marshal(s.Expr)
	}
	type wire StopWhenSpec
	return json.Marshal(wire(s))
}

func (s StopWhenSpec) validate() error {
	if strings.TrimSpace(s.Expr) == "" {
		return fmt.Errorf("stop_when.expr is required")
	}
	return s.validateOptional()
}

func (s StopWhenSpec) validateOptional() error {
	if !s.OnEvalError.Valid() {
		return fmt.Errorf("stop_when.on_eval_error must be fail or exit")
	}
	return nil
}

func rejectUnknownStopWhenYAMLFields(value *yaml.Node) error {
	seen := make(map[string]struct{}, len(value.Content)/2)
	for index := 0; index < len(value.Content); index += 2 {
		key := strings.TrimSpace(value.Content[index].Value)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("stop_when.%s is duplicated", key)
		}
		seen[key] = struct{}{}
		if key != "expr" && key != "on_eval_error" {
			return fmt.Errorf("stop_when.%s is unknown", key)
		}
	}
	return nil
}
