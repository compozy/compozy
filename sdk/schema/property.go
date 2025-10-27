package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/schema"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// PropertyBuilder constructs named object properties backed by Schema builders.
type PropertyBuilder struct {
	name        string
	required    bool
	schema      *Builder
	errors      []error
	typeDefined bool
}

// NewProperty creates a property builder for the given property name.
func NewProperty(name string) *PropertyBuilder {
	return &PropertyBuilder{
		name:   strings.TrimSpace(name),
		errors: make([]error, 0),
	}
}

// WithType selects the JSON Schema type for the property.
func (b *PropertyBuilder) WithType(typ string) *PropertyBuilder {
	if b == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case typeObject:
		b.schema = NewObject()
	case typeString:
		b.schema = NewString()
	case typeNumber:
		b.schema = NewNumber()
	case typeInteger:
		b.schema = NewInteger()
	case typeBoolean:
		b.schema = NewBoolean()
	case typeArray:
		b.schema = NewArray(nil)
	default:
		b.errors = append(b.errors, fmt.Errorf("unsupported property type %q", typ))
		return b
	}
	b.typeDefined = true
	return b
}

// WithDescription sets the description for the property.
func (b *PropertyBuilder) WithDescription(desc string) *PropertyBuilder {
	if b == nil {
		return nil
	}
	if b.schema == nil {
		b.errors = append(b.errors, fmt.Errorf("property type must be defined before setting description"))
		return b
	}
	b.schema.WithDescription(desc)
	return b
}

// WithDefault assigns a default value to the property.
func (b *PropertyBuilder) WithDefault(value any) *PropertyBuilder {
	if b == nil {
		return nil
	}
	if b.schema == nil {
		b.errors = append(b.errors, fmt.Errorf("property type must be defined before setting default"))
		return b
	}
	b.schema.WithDefault(value)
	return b
}

// Required marks the property as required on the parent object.
func (b *PropertyBuilder) Required() *PropertyBuilder {
	if b == nil {
		return nil
	}
	b.required = true
	return b
}

// AddProperty forwards to the underlying schema builder when the property type is object.
func (b *PropertyBuilder) AddProperty(name string, prop *Builder) *PropertyBuilder {
	if b == nil {
		return nil
	}
	if b.schema == nil {
		b.errors = append(b.errors, fmt.Errorf("property type must be object before adding nested properties"))
		return b
	}
	b.schema.AddProperty(name, prop)
	return b
}

// RequireProperty forwards required property settings to the underlying builder.
func (b *PropertyBuilder) RequireProperty(name string) *PropertyBuilder {
	if b == nil {
		return nil
	}
	if b.schema == nil {
		b.errors = append(b.errors, fmt.Errorf("property type must be object before requiring nested properties"))
		return b
	}
	b.schema.RequireProperty(name)
	return b
}

// WithMinLength proxies string minimum length to the schema builder.
func (b *PropertyBuilder) WithMinLength(minValue int) *PropertyBuilder {
	if b == nil || b.schema == nil {
		return b
	}
	b.schema.WithMinLength(minValue)
	return b
}

// WithMaxLength proxies string maximum length to the schema builder.
func (b *PropertyBuilder) WithMaxLength(maxValue int) *PropertyBuilder {
	if b == nil || b.schema == nil {
		return b
	}
	b.schema.WithMaxLength(maxValue)
	return b
}

// WithPattern forwards string pattern configuration.
func (b *PropertyBuilder) WithPattern(pattern string) *PropertyBuilder {
	if b == nil || b.schema == nil {
		return b
	}
	b.schema.WithPattern(pattern)
	return b
}

// WithEnum forwards enum configuration to the schema builder.
func (b *PropertyBuilder) WithEnum(values ...string) *PropertyBuilder {
	if b == nil || b.schema == nil {
		return b
	}
	b.schema.WithEnum(values...)
	return b
}

// WithMinimum forwards minimum configuration to number schemas.
func (b *PropertyBuilder) WithMinimum(minValue float64) *PropertyBuilder {
	if b == nil || b.schema == nil {
		return b
	}
	b.schema.WithMinimum(minValue)
	return b
}

// WithMaximum forwards maximum configuration to number schemas.
func (b *PropertyBuilder) WithMaximum(maxValue float64) *PropertyBuilder {
	if b == nil || b.schema == nil {
		return b
	}
	b.schema.WithMaximum(maxValue)
	return b
}

// WithMinItems forwards minimum items configuration to array schemas.
func (b *PropertyBuilder) WithMinItems(minValue int) *PropertyBuilder {
	if b == nil || b.schema == nil {
		return b
	}
	b.schema.WithMinItems(minValue)
	return b
}

// WithMaxItems forwards maximum items configuration to array schemas.
func (b *PropertyBuilder) WithMaxItems(maxValue int) *PropertyBuilder {
	if b == nil || b.schema == nil {
		return b
	}
	b.schema.WithMaxItems(maxValue)
	return b
}

// WithItems forwards array item configuration to the schema builder.
func (b *PropertyBuilder) WithItems(itemType *Builder) *PropertyBuilder {
	if b == nil {
		return nil
	}
	if b.schema == nil {
		b.errors = append(b.errors, fmt.Errorf("property type must be array before configuring items"))
		return b
	}
	b.schema.WithItems(itemType)
	return b
}

// Build finalizes the property definition and returns a schema property.
func (b *PropertyBuilder) Build(ctx context.Context) (*schema.Property, error) {
	if b == nil {
		return nil, fmt.Errorf("property builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	collected := make([]error, 0, len(b.errors)+4)
	collected = append(collected, b.errors...)
	if err := validate.NonEmpty(ctx, "property name", b.name); err != nil {
		collected = append(collected, err)
	}
	if !b.typeDefined || b.schema == nil {
		collected = append(collected, fmt.Errorf("property %q type must be defined", b.name))
	}
	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	schemaDef, err := b.schema.Build(ctx)
	if err != nil {
		return nil, err
	}
	return &schema.Property{
		Name:     b.name,
		Schema:   schemaDef,
		Required: b.required,
	}, nil
}
