package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func decodeSettingsSkillsUpdate(
	reader io.Reader,
	scope settingspkg.ScopeKind,
) (*contract.SettingsSkillsConfigPayload, *settingspkg.SkillSourcesOverride, error) {
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("decode skills settings request: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, nil, errors.New("decode skills settings request: multiple JSON values")
		}
		return nil, nil, fmt.Errorf("decode skills settings request: %w", err)
	}

	if scope == settingspkg.ScopeWorkspace {
		override, err := decodeWorkspaceSkillSourcesOverride(raw)
		return nil, override, err
	}

	for key := range raw {
		if key != "config" {
			return nil, nil, fmt.Errorf("decode skills settings request: unknown field %q", key)
		}
	}
	configData, ok := raw["config"]
	if !ok {
		return nil, nil, errors.New("skills.config is required")
	}
	var payload contract.SettingsSkillsConfigPayload
	if err := decodeStrictSettingsJSON(configData, &payload); err != nil {
		return nil, nil, fmt.Errorf("decode skills settings request: %w", err)
	}
	return &payload, nil, nil
}

func decodeWorkspaceSkillSourcesOverride(raw map[string]json.RawMessage) (*settingspkg.SkillSourcesOverride, error) {
	if forbidden := forbiddenWorkspaceSkillsField(raw); forbidden != "" {
		return nil, workspaceSkillsFieldForbidden(forbidden)
	}
	overrideData, ok := raw["override"]
	if !ok {
		return nil, errors.New("skills.override is required at workspace scope")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(overrideData, &fields); err != nil {
		return nil, fmt.Errorf("decode skills.override: %w", err)
	}
	for field := range fields {
		if field != "sources" && field != "custom_sources" {
			return nil, workspaceSkillsFieldForbidden(field)
		}
	}
	result := &settingspkg.SkillSourcesOverride{}
	if data, present := fields["sources"]; present {
		value, err := decodeOptionalStringList(data)
		if err != nil {
			return nil, fmt.Errorf("decode skills.override.sources: %w", err)
		}
		result.Sources = value
	}
	if data, present := fields["custom_sources"]; present {
		value, err := decodeOptionalStringList(data)
		if err != nil {
			return nil, fmt.Errorf("decode skills.override.custom_sources: %w", err)
		}
		result.CustomSources = value
	}
	return result, nil
}

func forbiddenWorkspaceSkillsField(raw map[string]json.RawMessage) string {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		if key != "override" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	if keys[0] != "config" {
		return keys[0]
	}
	var configFields map[string]json.RawMessage
	if err := json.Unmarshal(raw["config"], &configFields); err != nil || len(configFields) == 0 {
		return "config"
	}
	nested := make([]string, 0, len(configFields))
	for field := range configFields {
		nested = append(nested, field)
	}
	sort.Strings(nested)
	return nested[0]
}

func decodeOptionalStringList(data []byte) (settingspkg.OptionalStringList, error) {
	var wire contract.OptionalStringList
	if err := json.Unmarshal(data, &wire); err != nil {
		return settingspkg.OptionalStringList{}, err
	}
	value := []string(nil)
	if !wire.Null {
		value = append([]string{}, wire.Value...)
	}
	return settingspkg.OptionalStringList{
		Present: wire.Present,
		Null:    wire.Null,
		Value:   value,
	}, nil
}

func decodeStrictSettingsJSON(data []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func workspaceSkillsFieldForbidden(field string) error {
	return &compozyconfig.SkillSourceValidationError{
		Code:    "workspace_scope_field_forbidden",
		Field:   strings.TrimSpace(field),
		Message: "only sources and custom_sources may be written at workspace scope",
	}
}
