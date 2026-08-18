package dsl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// StrategyKind selects the completion rule for a fan-out scope.
type StrategyKind string

const (
	StrategyWaitAll    StrategyKind = "wait_all"
	StrategyFailFast   StrategyKind = "fail_fast"
	StrategyBestEffort StrategyKind = "best_effort"
	StrategyRace       StrategyKind = "race"
)

func (k StrategyKind) Valid() bool {
	switch k {
	case StrategyWaitAll, StrategyFailFast, StrategyBestEffort, StrategyRace:
		return true
	default:
		return false
	}
}

// MissingPolicy records whether incomplete coverage is author-approved.
type MissingPolicy string

const MissingAcceptable MissingPolicy = "acceptable"

func (p MissingPolicy) Valid() bool { return p == "" || p == MissingAcceptable }

type ThresholdKind string

const (
	ThresholdPercent ThresholdKind = "percent"
	ThresholdCount   ThresholdKind = "count"
)

// StrategyThreshold is exactly either an integer percent string or a count object.
type StrategyThreshold struct {
	Kind    ThresholdKind
	Percent int
	Count   int
}

var strategyPercentPattern = regexp.MustCompile(`^([0-9]+)%$`)

func (t StrategyThreshold) Validate() error {
	switch t.Kind {
	case ThresholdPercent:
		if t.Percent < 1 || t.Percent > 100 || t.Count != 0 {
			return fmt.Errorf("strategy threshold percent must be between 1%% and 100%%")
		}
	case ThresholdCount:
		if t.Count < 1 || t.Percent != 0 {
			return fmt.Errorf("strategy threshold count must be positive")
		}
	default:
		return fmt.Errorf("strategy threshold kind is invalid")
	}
	return nil
}

func (t *StrategyThreshold) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return fmt.Errorf("strategy threshold is required")
	}
	if value.Kind == yaml.ScalarNode {
		return t.decodePercent(value.Value)
	}
	if value.Kind != yaml.MappingNode || len(value.Content) != 2 || value.Content[0].Value != "count" {
		return fmt.Errorf("strategy threshold must be an NN%% string or {count: N}")
	}
	var count int
	if err := value.Content[1].Decode(&count); err != nil {
		return fmt.Errorf("decode strategy threshold count: %w", err)
	}
	*t = StrategyThreshold{Kind: ThresholdCount, Count: count}
	return t.Validate()
}

func (t StrategyThreshold) MarshalYAML() (any, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if t.Kind == ThresholdPercent {
		return strconv.Itoa(t.Percent) + "%", nil
	}
	return struct {
		Count int `yaml:"count"`
	}{Count: t.Count}, nil
}

func (t *StrategyThreshold) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("strategy threshold is required")
	}
	if trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return fmt.Errorf("decode strategy threshold percent: %w", err)
		}
		return t.decodePercent(value)
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	var value struct {
		Count int `json:"count"`
	}
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("decode strategy threshold count: %w", err)
	}
	if err := requireJSONEOF(decoder, "strategy threshold"); err != nil {
		return err
	}
	*t = StrategyThreshold{Kind: ThresholdCount, Count: value.Count}
	return t.Validate()
}

func (t StrategyThreshold) MarshalJSON() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if t.Kind == ThresholdPercent {
		return json.Marshal(strconv.Itoa(t.Percent) + "%")
	}
	return json.Marshal(struct {
		Count int `json:"count"`
	}{Count: t.Count})
}

func (t *StrategyThreshold) decodePercent(value string) error {
	match := strategyPercentPattern.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return fmt.Errorf("strategy threshold must be an NN%% string or {count: N}")
	}
	percent, err := strconv.Atoi(match[1])
	if err != nil {
		return fmt.Errorf("decode strategy threshold percent: %w", err)
	}
	*t = StrategyThreshold{Kind: ThresholdPercent, Percent: percent}
	return t.Validate()
}

// StrategySpec declares one fan-out join strategy.
type StrategySpec struct {
	Kind      StrategyKind       `json:"kind"                yaml:"kind"`
	Threshold *StrategyThreshold `json:"threshold,omitempty" yaml:"threshold,omitempty"`
	Missing   MissingPolicy      `json:"missing,omitempty"   yaml:"missing,omitempty"`
}

func (s StrategySpec) ValidateShape() error {
	if !s.Kind.Valid() {
		return fmt.Errorf("strategy kind is invalid")
	}
	if !s.Missing.Valid() {
		return fmt.Errorf("strategy missing policy is invalid")
	}
	if s.Threshold != nil {
		if err := s.Threshold.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (s *StrategySpec) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return fmt.Errorf("strategy is required")
	}
	if value.Kind == yaml.ScalarNode {
		*s = StrategySpec{Kind: StrategyKind(strings.TrimSpace(value.Value))}
		return s.validateShorthand()
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("strategy must be a string or object")
	}
	if err := rejectUnknownMappingKeys(
		value,
		map[string]struct{}{"kind": {}, "threshold": {}, "missing": {}},
		"strategy",
	); err != nil {
		return err
	}
	type wire StrategySpec
	var decoded wire
	if err := value.Decode(&decoded); err != nil {
		return fmt.Errorf("decode strategy: %w", err)
	}
	*s = StrategySpec(decoded)
	return s.ValidateShape()
}

func (s StrategySpec) MarshalYAML() (any, error) {
	if err := s.ValidateShape(); err != nil {
		return nil, err
	}
	if s.Threshold == nil && s.Missing == "" && s.Kind != StrategyBestEffort {
		return string(s.Kind), nil
	}
	type wire StrategySpec
	return wire(s), nil
}

func (s *StrategySpec) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("strategy is required")
	}
	if trimmed[0] == '"' {
		var kind StrategyKind
		if err := json.Unmarshal(trimmed, &kind); err != nil {
			return fmt.Errorf("decode strategy shorthand: %w", err)
		}
		*s = StrategySpec{Kind: kind}
		return s.validateShorthand()
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	type wire StrategySpec
	var decoded wire
	if err := decoder.Decode(&decoded); err != nil {
		return fmt.Errorf("decode strategy: %w", err)
	}
	if err := requireJSONEOF(decoder, "strategy"); err != nil {
		return err
	}
	*s = StrategySpec(decoded)
	return s.ValidateShape()
}

func (s StrategySpec) MarshalJSON() ([]byte, error) {
	if err := s.ValidateShape(); err != nil {
		return nil, err
	}
	if s.Threshold == nil && s.Missing == "" && s.Kind != StrategyBestEffort {
		return json.Marshal(s.Kind)
	}
	type wire StrategySpec
	return json.Marshal(wire(s))
}

func (s StrategySpec) validateShorthand() error {
	if s.Kind == StrategyBestEffort || !s.Kind.Valid() {
		return fmt.Errorf("strategy shorthand must be wait_all, fail_fast, or race")
	}
	return nil
}

func rejectUnknownMappingKeys(node *yaml.Node, allowed map[string]struct{}, field string) error {
	for index := 0; index+1 < len(node.Content); index += 2 {
		key := node.Content[index].Value
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("%s contains unknown field %q", field, key)
		}
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder, field string) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode %s: trailing value", field)
		}
		return fmt.Errorf("decode %s trailing value: %w", field, err)
	}
	return nil
}
