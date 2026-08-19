package contract

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
	"gopkg.in/yaml.v3"
)

// LoopDefinitionDocument is the public compozy.loop/v1 authoring document.
// It deliberately shares the DSL's complete authoring shape so public transports
// cannot lag behind fields accepted by the parser.
type LoopDefinitionDocument dsl.Definition

// NewLoopDefinitionDocument returns a caller-owned public authoring document.
func NewLoopDefinitionDocument(definition dsl.Definition) (LoopDefinitionDocument, error) {
	cloned, err := cloneLoopDefinition(definition)
	if err != nil {
		return LoopDefinitionDocument{}, fmt.Errorf("contract: encode loop definition: %w", err)
	}
	return LoopDefinitionDocument(cloned), nil
}

// Decode returns a caller-owned canonical DSL definition.
func (d LoopDefinitionDocument) Decode(target *dsl.Definition) error {
	if target == nil {
		return fmt.Errorf("contract: loop definition target is required")
	}
	cloned, err := cloneLoopDefinition(dsl.Definition(d))
	if err != nil {
		return fmt.Errorf("contract: decode loop definition: %w", err)
	}
	*target = cloned
	return nil
}

// MarshalJSON keeps YAML-inline extension fields on every DSL object visible on
// JSON transports instead of dropping them through their json:"-" struct tags.
func (d LoopDefinitionDocument) MarshalJSON() ([]byte, error) {
	data, err := dsl.Serialize(dsl.Definition(d))
	if err != nil {
		return nil, fmt.Errorf("contract: serialize loop definition: %w", err)
	}
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("contract: decode serialized loop definition: %w", err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("contract: encode loop definition JSON: %w", err)
	}
	return encoded, nil
}

// UnmarshalJSON accepts the same document shape as dsl.Parse, including inline
// extension fields and polymorphic authoring values.
func (d *LoopDefinitionDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return fmt.Errorf("contract: loop definition document is required")
	}
	definition, err := dsl.Parse(data)
	if err != nil {
		return fmt.Errorf("contract: decode loop definition JSON: %w", err)
	}
	*d = LoopDefinitionDocument(definition)
	return nil
}

func cloneLoopDefinition(definition dsl.Definition) (dsl.Definition, error) {
	data, err := dsl.Serialize(definition)
	if err != nil {
		return dsl.Definition{}, err
	}
	cloned, err := dsl.Parse(data)
	if err != nil {
		return dsl.Definition{}, err
	}
	return cloned, nil
}
