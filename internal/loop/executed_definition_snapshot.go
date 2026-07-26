package loop

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strings"

	"github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/loop/dsl"
)

const executedDefinitionSnapshotFormatVersion = 1

type executedDefinitionSnapshot struct {
	FormatVersion        int                                     `json:"format_version"`
	DefinitionVersion    int                                     `json:"definition_version"`
	Definition           dsl.Definition                          `json:"definition"`
	Defaults             ResolvedDefaults                        `json:"resolved_defaults"`
	EffectiveConfig      EffectiveConfig                         `json:"effective_config"`
	ToolSchemas          map[string]ToolSchemaSnapshot           `json:"tool_schemas"`
	WatchEventsContracts map[hooks.HookEvent]WatchEventsContract `json:"watch_events_contracts"`
	TemplateSources      map[string]string                       `json:"template_sources"`
	ConditionSources     map[string]string                       `json:"condition_sources"`
}

// BuildExecutedDefinitionSnapshot creates the only content-addressed artifact executable after Start.
func BuildExecutedDefinitionSnapshot(
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
) (json.RawMessage, string, error) {
	if resolved == nil {
		return nil, "", fmt.Errorf("%w: resolved definition is required", ErrValidation)
	}
	if !resolved.compiled {
		return nil, "", fmt.Errorf("%w: definition must be compiled before snapshotting", ErrValidation)
	}
	definition := resolved.Definition
	definition.Normalize()
	version := resolved.DefinitionVersion
	if version == 0 {
		version = definition.Meta.Version
	}
	contracts := resolved.WatchEventsContracts
	if contracts == nil {
		contracts = referencedWatchEventsContracts(definition, SupportedWatchEvents())
	}
	envelope := executedDefinitionSnapshot{
		FormatVersion:        executedDefinitionSnapshotFormatVersion,
		DefinitionVersion:    version,
		Definition:           definition,
		Defaults:             resolved.Defaults,
		EffectiveConfig:      cloneEffectiveConfig(effective),
		ToolSchemas:          cloneToolSchemaSnapshots(resolved.ToolSchemas),
		WatchEventsContracts: cloneWatchEventsContracts(contracts),
		TemplateSources:      resolvedTemplateSources(resolved),
		ConditionSources:     resolvedConditionSources(resolved),
	}
	if err := validateExecutedDefinitionSnapshot(&envelope); err != nil {
		return nil, "", err
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, "", fmt.Errorf("loop: marshal executed definition snapshot: %w", err)
	}
	canonical, err := canonicalExecutedDefinitionSnapshot(data)
	if err != nil {
		return nil, "", err
	}
	return json.RawMessage(canonical), executedDefinitionDigest(canonical), nil
}

func canonicalExecutedDefinitionSnapshot(data []byte) ([]byte, error) {
	var canonical executedDefinitionSnapshot
	if err := json.Unmarshal(data, &canonical); err != nil {
		return nil, fmt.Errorf("loop: canonicalize executed definition snapshot: %w", err)
	}
	if err := validateExecutedDefinitionSnapshot(&canonical); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("loop: marshal canonical executed definition snapshot: %w", err)
	}
	return encoded, nil
}

// LoadExecutedDefinitionSnapshot verifies and purely hydrates one Run-owned execution artifact.
func LoadExecutedDefinitionSnapshot(
	raw json.RawMessage,
	expectedDigest string,
) (*ResolvedDefinition, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("%w: executed definition snapshot is required", ErrValidation)
	}
	expectedDigest = strings.TrimSpace(expectedDigest)
	if expectedDigest == "" {
		return nil, fmt.Errorf("%w: executed definition digest is required", ErrValidation)
	}
	var envelope executedDefinitionSnapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("loop: decode executed definition snapshot: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("%w: executed definition snapshot has trailing JSON", ErrValidation)
		}
		return nil, fmt.Errorf("loop: decode executed definition snapshot tail: %w", err)
	}
	if err := validateExecutedDefinitionSnapshot(&envelope); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("loop: marshal hydrated definition snapshot: %w", err)
	}
	digest := executedDefinitionDigest(canonical)
	if digest != expectedDigest {
		return nil, fmt.Errorf(
			"%w: executed definition digest %q does not match %q",
			ErrValidation,
			digest,
			expectedDigest,
		)
	}
	resolved, err := hydrateExecutedDefinitionSnapshot(&envelope)
	if err != nil {
		return nil, err
	}
	if !maps.Equal(resolvedTemplateSources(resolved), envelope.TemplateSources) {
		return nil, fmt.Errorf("%w: executed definition template manifest changed", ErrValidation)
	}
	if !maps.Equal(resolvedConditionSources(resolved), envelope.ConditionSources) {
		return nil, fmt.Errorf("%w: executed definition condition manifest changed", ErrValidation)
	}
	return resolved, nil
}

func executedDefinitionDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cloneEffectiveConfig(config EffectiveConfig) EffectiveConfig {
	config.EnabledChecks = cloneRawMessage(config.EnabledChecks)
	return config
}

func cloneToolSchemaSnapshots(
	snapshots map[string]ToolSchemaSnapshot,
) map[string]ToolSchemaSnapshot {
	cloned := make(map[string]ToolSchemaSnapshot, len(snapshots))
	for kind, snapshot := range snapshots {
		snapshot.InputSchema = cloneRawMessage(snapshot.InputSchema)
		snapshot.OutputSchema = cloneRawMessage(snapshot.OutputSchema)
		cloned[kind] = snapshot
	}
	return cloned
}
