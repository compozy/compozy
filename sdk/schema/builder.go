package schema

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/compozy/compozy/engine/core"
	engschema "github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

const (
	typeObject  = "object"
	typeString  = "string"
	typeNumber  = "number"
	typeInteger = "integer"
	typeBoolean = "boolean"
	typeArray   = "array"
)

// Builder constructs JSON Schema definitions using a fluent API while aggregating validation errors.
type Builder struct {
	schema engschema.Schema
	errors []error
}

// NewObject creates a builder for an object schema.
func NewObject() *Builder {
	return &Builder{
		schema: engschema.Schema{
			"type":       typeObject,
			"properties": map[string]any{},
		},
		errors: make([]error, 0),
	}
}

// NewString creates a builder for a string schema.
func NewString() *Builder {
	return newPrimitiveBuilder(typeString)
}

// NewNumber creates a builder for a number schema.
func NewNumber() *Builder {
	return newPrimitiveBuilder(typeNumber)
}

// NewInteger creates a builder for an integer schema.
func NewInteger() *Builder {
	return newPrimitiveBuilder(typeInteger)
}

// NewBoolean creates a builder for a boolean schema.
func NewBoolean() *Builder {
	return newPrimitiveBuilder(typeBoolean)
}

// NewArray creates a builder for an array schema with the provided item definition.
func NewArray(itemType *Builder) *Builder {
	builder := &Builder{
		schema: engschema.Schema{
			"type": typeArray,
		},
		errors: make([]error, 0),
	}
	if itemType != nil {
		return builder.WithItems(itemType)
	}
	return builder
}

// AddProperty registers a named property on an object schema.
func (b *Builder) AddProperty(name string, prop *Builder) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("property name cannot be empty"))
		return b
	}
	if !b.isType(typeObject) {
		b.errors = append(b.errors, fmt.Errorf("properties can only be added to object schemas"))
		return b
	}
	if prop == nil {
		b.errors = append(b.errors, fmt.Errorf("property %q schema cannot be nil", trimmed))
		return b
	}
	propertySchema, err := prop.cloneSchema()
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("failed to clone property %q: %w", trimmed, err))
		return b
	}
	b.errors = append(b.errors, prop.errors...)
	props := b.ensureProperties()
	props[trimmed] = map[string]any(propertySchema)
	return b
}

// RequireProperty marks a property as required on an object schema.
func (b *Builder) RequireProperty(name string) *Builder {
	if b == nil {
		return nil
	}
	if !b.isType(typeObject) {
		b.errors = append(b.errors, fmt.Errorf("required properties can only be set on object schemas"))
		return b
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("required property name cannot be empty"))
		return b
	}
	required := b.ensureRequired()
	for _, existing := range required {
		if existing == trimmed {
			return b
		}
	}
	b.schema["required"] = append(required, trimmed)
	return b
}

// WithMinLength sets the minimum length for string schemas.
func (b *Builder) WithMinLength(minValue int) *Builder {
	if b == nil {
		return nil
	}
	if !b.isType(typeString) {
		b.errors = append(b.errors, fmt.Errorf("minLength is only valid for string schemas"))
		return b
	}
	if minValue < 0 {
		b.errors = append(b.errors, fmt.Errorf("minLength must be non-negative"))
		return b
	}
	b.schema["minLength"] = minValue
	return b
}

// WithMaxLength sets the maximum length for string schemas.
func (b *Builder) WithMaxLength(maxValue int) *Builder {
	if b == nil {
		return nil
	}
	if !b.isType(typeString) {
		b.errors = append(b.errors, fmt.Errorf("maxLength is only valid for string schemas"))
		return b
	}
	if maxValue < 0 {
		b.errors = append(b.errors, fmt.Errorf("maxLength must be non-negative"))
		return b
	}
	b.schema["maxLength"] = maxValue
	return b
}

// WithPattern sets the regex pattern for string schemas.
func (b *Builder) WithPattern(pattern string) *Builder {
	if b == nil {
		return nil
	}
	if !b.isType(typeString) {
		b.errors = append(b.errors, fmt.Errorf("pattern is only valid for string schemas"))
		return b
	}
	if _, err := regexp.Compile(pattern); err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid pattern: %w", err))
		return b
	}
	b.schema["pattern"] = pattern
	return b
}

// WithEnum restricts string schemas to the provided values.
func (b *Builder) WithEnum(values ...string) *Builder {
	if b == nil {
		return nil
	}
	if !b.isType(typeString) {
		b.errors = append(b.errors, fmt.Errorf("enum is only valid for string schemas"))
		return b
	}
	if len(values) == 0 {
		b.errors = append(b.errors, fmt.Errorf("enum requires at least one value"))
		return b
	}
	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			b.errors = append(b.errors, fmt.Errorf("enum values cannot be empty"))
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		unique = append(unique, trimmed)
	}
	if len(unique) == 0 {
		b.errors = append(b.errors, fmt.Errorf("enum requires non-empty values"))
		return b
	}
	b.schema["enum"] = unique
	return b
}

// WithMinimum sets the minimum value for number or integer schemas.
func (b *Builder) WithMinimum(minValue float64) *Builder {
	if b == nil {
		return nil
	}
	if !b.isNumeric() {
		b.errors = append(b.errors, fmt.Errorf("minimum is only valid for number or integer schemas"))
		return b
	}
	b.schema["minimum"] = minValue
	return b
}

// WithMaximum sets the maximum value for number or integer schemas.
func (b *Builder) WithMaximum(maxValue float64) *Builder {
	if b == nil {
		return nil
	}
	if !b.isNumeric() {
		b.errors = append(b.errors, fmt.Errorf("maximum is only valid for number or integer schemas"))
		return b
	}
	b.schema["maximum"] = maxValue
	return b
}

// WithMinItems sets the minimum number of items for array schemas.
func (b *Builder) WithMinItems(minValue int) *Builder {
	if b == nil {
		return nil
	}
	if !b.isType(typeArray) {
		b.errors = append(b.errors, fmt.Errorf("minItems is only valid for array schemas"))
		return b
	}
	if minValue < 0 {
		b.errors = append(b.errors, fmt.Errorf("minItems must be non-negative"))
		return b
	}
	b.schema["minItems"] = minValue
	return b
}

// WithMaxItems sets the maximum number of items for array schemas.
func (b *Builder) WithMaxItems(maxValue int) *Builder {
	if b == nil {
		return nil
	}
	if !b.isType(typeArray) {
		b.errors = append(b.errors, fmt.Errorf("maxItems is only valid for array schemas"))
		return b
	}
	if maxValue < 0 {
		b.errors = append(b.errors, fmt.Errorf("maxItems must be non-negative"))
		return b
	}
	b.schema["maxItems"] = maxValue
	return b
}

// WithItems replaces the item schema for array definitions.
func (b *Builder) WithItems(itemType *Builder) *Builder {
	if b == nil {
		return nil
	}
	if !b.isType(typeArray) {
		b.errors = append(b.errors, fmt.Errorf("items can only be configured for array schemas"))
		return b
	}
	if itemType == nil {
		b.errors = append(b.errors, fmt.Errorf("array item schema cannot be nil"))
		return b
	}
	itemSchema, err := itemType.cloneSchema()
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("failed to clone array item schema: %w", err))
		return b
	}
	b.errors = append(b.errors, itemType.errors...)
	b.schema["items"] = map[string]any(itemSchema)
	return b
}

// WithDefault assigns a default value to the schema.
func (b *Builder) WithDefault(value any) *Builder {
	if b == nil {
		return nil
	}
	b.schema["default"] = value
	return b
}

// WithDescription adds a human readable description to the schema.
func (b *Builder) WithDescription(desc string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(desc)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("description cannot be empty"))
		return b
	}
	b.schema["description"] = trimmed
	return b
}

// WithRef converts the builder into a schema reference.
func (b *Builder) WithRef(schemaID string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(schemaID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("schema reference ID cannot be empty"))
		return b
	}
	keys := []string{
		"type",
		"properties",
		"required",
		"items",
		"enum",
		"minLength",
		"maxLength",
		"pattern",
		"minimum",
		"maximum",
		"minItems",
		"maxItems",
	}
	for _, key := range keys {
		delete(b.schema, key)
	}
	b.schema["$ref"] = trimmed
	return b
}

// ValidateSchema compiles the schema to ensure it is structurally valid.
func (b *Builder) ValidateSchema(ctx context.Context) error {
	sch, err := b.build(ctx)
	if err != nil {
		return err
	}
	if sch == nil {
		return nil
	}
	_, compileErr := sch.Compile(ctx)
	if compileErr != nil {
		return fmt.Errorf("schema compilation failed: %w", compileErr)
	}
	return nil
}

// TestAgainstSample validates a sample payload against the built schema.
func (b *Builder) TestAgainstSample(ctx context.Context, sample any) error {
	sch, err := b.build(ctx)
	if err != nil {
		return err
	}
	if sch == nil {
		return nil
	}
	result, validateErr := sch.Validate(ctx, sample)
	if validateErr != nil {
		return validateErr
	}
	_ = result
	return nil
}

// Build finalizes the schema and returns a deep copy ready for use.
func (b *Builder) Build(ctx context.Context) (*engschema.Schema, error) {
	return b.build(ctx)
}

func (b *Builder) build(ctx context.Context) (*engschema.Schema, error) {
	if b == nil {
		return nil, fmt.Errorf("schema builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	logger.FromContext(ctx).Debug("building schema definition")
	collected := make([]error, 0, len(b.errors)+8)
	collected = append(collected, b.errors...)
	collected = append(collected, b.validateStructure(ctx)...)
	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned, err := core.DeepCopy(b.schema)
	if err != nil {
		return nil, fmt.Errorf("failed to clone schema: %w", err)
	}
	return &cloned, nil
}

func (b *Builder) validateStructure(ctx context.Context) []error {
	errs := make([]error, 0, 4)
	if ctx == nil {
		return append(errs, fmt.Errorf("context is required"))
	}
	switch b.schema["type"] {
	case typeString:
		errs = append(errs, b.validateStringBounds(ctx)...)
	case typeNumber, typeInteger:
		errs = append(errs, b.validateNumberBounds(ctx)...)
	case typeArray:
		errs = append(errs, b.validateArrayBounds(ctx)...)
	case typeObject, nil:
		// object and ref validations handled below
	case typeBoolean:
		// booleans have no additional structural validation
	default:
		errs = append(errs, fmt.Errorf("unsupported schema type %v", b.schema["type"]))
	}
	if _, hasRef := b.schema["$ref"]; hasRef {
		if err := validate.NonEmpty(ctx, "schema reference", fmt.Sprint(b.schema["$ref"])); err != nil {
			errs = append(errs, err)
		}
	}
	if b.isType(typeObject) {
		errs = append(errs, b.validateObject(ctx)...)
	}
	return errs
}

func (b *Builder) validateStringBounds(_ context.Context) []error {
	errs := make([]error, 0, 2)
	minValue, minOK := b.schema["minLength"].(int)
	maxValue, maxOK := b.schema["maxLength"].(int)
	if minOK && maxOK && minValue > maxValue {
		errs = append(errs, fmt.Errorf("minLength cannot exceed maxLength"))
	}
	if enumValues, ok := b.schema["enum"].([]string); ok {
		if len(enumValues) == 0 {
			errs = append(errs, fmt.Errorf("enum requires at least one value"))
		}
	}
	return errs
}

func (b *Builder) validateNumberBounds(_ context.Context) []error {
	errs := make([]error, 0, 1)
	minValue, minOK := b.schema["minimum"].(float64)
	maxValue, maxOK := b.schema["maximum"].(float64)
	if minOK && maxOK && minValue > maxValue {
		errs = append(errs, fmt.Errorf("minimum cannot exceed maximum"))
	}
	return errs
}

func (b *Builder) validateArrayBounds(_ context.Context) []error {
	errs := make([]error, 0, 2)
	minValue, minOK := b.schema["minItems"].(int)
	maxValue, maxOK := b.schema["maxItems"].(int)
	if minOK && maxOK && minValue > maxValue {
		errs = append(errs, fmt.Errorf("minItems cannot exceed maxItems"))
	}
	if _, ok := b.schema["items"]; !ok {
		errs = append(errs, fmt.Errorf("array schema must define items"))
	}
	return errs
}

func (b *Builder) validateObject(ctx context.Context) []error {
	errs := make([]error, 0, 4)
	props, ok := b.schema["properties"].(map[string]any)
	if !ok {
		props = make(map[string]any)
		b.schema["properties"] = props
	}
	for name := range props {
		if err := validate.NonEmpty(ctx, "property name", name); err != nil {
			errs = append(errs, err)
		}
	}
	required, ok := b.schema["required"].([]string)
	if !ok {
		return errs
	}
	for _, field := range required {
		if _, exists := props[field]; !exists {
			errs = append(errs, fmt.Errorf("required property %q is not defined", field))
		}
	}
	return errs
}

func (b *Builder) ensureProperties() map[string]any {
	props, ok := b.schema["properties"].(map[string]any)
	if !ok {
		props = make(map[string]any)
		b.schema["properties"] = props
	}
	return props
}

func (b *Builder) ensureRequired() []string {
	required, ok := b.schema["required"].([]string)
	if !ok {
		required = make([]string, 0)
	}
	return required
}

func (b *Builder) isType(expected string) bool {
	v, ok := b.schema["type"].(string)
	if !ok {
		return false
	}
	return v == expected
}

func (b *Builder) isNumeric() bool {
	t, ok := b.schema["type"].(string)
	if !ok {
		return false
	}
	return t == typeNumber || t == typeInteger
}

func (b *Builder) cloneSchema() (engschema.Schema, error) {
	copied, err := core.DeepCopy(b.schema)
	if err != nil {
		return nil, err
	}
	return copied, nil
}

func newPrimitiveBuilder(typ string) *Builder {
	return &Builder{
		schema: engschema.Schema{"type": typ},
		errors: make([]error, 0),
	}
}

func filterErrors(errs []error) []error {
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return filtered
}
