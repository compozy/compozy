package schema

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

// Property represents a named JSON Schema property with optional required state.
type Property struct {
	Name     string  `json:"name"`
	Schema   *Schema `json:"schema"`
	Required bool    `json:"required"`
}

// Clone creates a deep copy of the property to avoid shared state between callers.
func (p *Property) Clone() (*Property, error) {
	if p == nil {
		return nil, nil
	}
	copied, err := core.DeepCopy(*p)
	if err != nil {
		return nil, fmt.Errorf("failed to clone property %q: %w", p.Name, err)
	}
	return &copied, nil
}
