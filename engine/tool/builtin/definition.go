package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
)

// Handler executes a builtin tool using the provided input payload.
type Handler func(ctx context.Context, input map[string]any) (core.Output, error)

// BuiltinDefinition captures the metadata and handler for a cp__ tool.
type BuiltinDefinition struct { //nolint:revive // Exported name follows PRD naming contract.
	ID            string
	Description   string
	InputSchema   *schema.Schema
	OutputSchema  *schema.Schema
	ArgsPrototype any
	Handler       Handler
}

// Validate ensures the definition is well formed before registration.
func (d BuiltinDefinition) Validate() error {
	if d.ID == "" {
		return errors.New("builtin definition requires id")
	}
	if d.Handler == nil {
		return fmt.Errorf("builtin %s missing handler", d.ID)
	}
	return nil
}

// Tool represents the methods required by the shared tool registry.
type Tool interface {
	Name() string
	Description() string
	Call(ctx context.Context, input string) (string, error)
	ParameterSchema() map[string]any
}

type BuiltinTool struct { //nolint:revive // External callers expect BuiltinTool identifier.
	definition BuiltinDefinition
}

// NewBuiltinTool creates a registry-compatible tool wrapper around definition.
func NewBuiltinTool(definition BuiltinDefinition) (*BuiltinTool, error) {
	if err := definition.Validate(); err != nil {
		return nil, err
	}
	return &BuiltinTool{definition: definition}, nil
}

func (b *BuiltinTool) Name() string {
	return b.definition.ID
}

func (b *BuiltinTool) Description() string {
	return b.definition.Description
}

func (b *BuiltinTool) Call(ctx context.Context, input string) (string, error) {
	payload := make(map[string]any)
	if input != "" {
		if err := json.Unmarshal([]byte(input), &payload); err != nil {
			return "", fmt.Errorf("failed to decode builtin input: %w", err)
		}
	}
	output, err := b.definition.Handler(ctx, payload)
	if err != nil {
		return "", err
	}
	if output == nil {
		output = core.Output{}
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to encode builtin output: %w", err)
	}
	return string(encoded), nil
}

// ArgsType exposes the definition argument prototype for schema generation.
func (b *BuiltinTool) ArgsType() any {
	return b.definition.ArgsPrototype
}

// Definition returns the underlying builtin definition.
func (b *BuiltinTool) Definition() BuiltinDefinition {
	return b.definition
}

// InputSchema exposes the builtin input schema so callers can advertise parameters accurately.
func (b *BuiltinTool) InputSchema() *schema.Schema {
	return b.definition.InputSchema
}

func (b *BuiltinTool) ParameterSchema() map[string]any {
	if b.definition.InputSchema == nil {
		return nil
	}
	source := map[string]any(*b.definition.InputSchema)
	copied, err := core.DeepCopy(source)
	if err != nil {
		return core.CloneMap(source)
	}
	return copied
}
