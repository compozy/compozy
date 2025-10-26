package schema

import (
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/schema"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilder_NewTypeConstructors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		builder  *Builder
		expected string
	}{
		{name: "object", builder: NewObject(), expected: typeObject},
		{name: "string", builder: NewString(), expected: typeString},
		{name: "number", builder: NewNumber(), expected: typeNumber},
		{name: "integer", builder: NewInteger(), expected: typeInteger},
		{name: "boolean", builder: NewBoolean(), expected: typeBoolean},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := tt.builder.Build(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tt.expected, (*result)["type"])
		})
	}
}

func TestBuilder_NewArray(t *testing.T) {
	t.Parallel()
	item := NewString().WithMinLength(1)
	array := NewArray(item).WithMinItems(1).WithMaxItems(5)
	result, err := array.Build(t.Context())
	require.NoError(t, err)
	assert.Equal(t, typeArray, (*result)["type"])
	items, ok := (*result)["items"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, typeString, items["type"])
	assert.Equal(t, 1, (*result)["minItems"])
	assert.Equal(t, 5, (*result)["maxItems"])
}

func TestBuilder_StringConstraints(t *testing.T) {
	t.Parallel()
	builder := NewString().
		WithMinLength(2).
		WithMaxLength(10).
		WithPattern(`^[A-Z]+$`).
		WithEnum("ONE", "TWO").
		WithDefault("ONE")
	result, err := builder.Build(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, (*result)["minLength"])
	assert.Equal(t, 10, (*result)["maxLength"])
	assert.Equal(t, `^[A-Z]+$`, (*result)["pattern"])
	assert.Equal(t, []string{"ONE", "TWO"}, (*result)["enum"])
	assert.Equal(t, "ONE", (*result)["default"])
}

func TestBuilder_NumberConstraints(t *testing.T) {
	t.Parallel()
	builder := NewNumber().WithMinimum(1.5).WithMaximum(9.5)
	result, err := builder.Build(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1.5, (*result)["minimum"])
	assert.Equal(t, 9.5, (*result)["maximum"])
}

func TestBuilder_ObjectProperties(t *testing.T) {
	t.Parallel()
	object := NewObject().
		AddProperty("name", NewString().WithMinLength(1)).
		AddProperty("age", NewInteger().WithMinimum(0)).
		RequireProperty("name")
	result, err := object.Build(t.Context())
	require.NoError(t, err)
	props := (*result)["properties"].(map[string]any)
	assert.Contains(t, props, "name")
	assert.Contains(t, props, "age")
	required := (*result)["required"].([]string)
	assert.Equal(t, []string{"name"}, required)
}

func TestBuilder_AddProperty_Errors(t *testing.T) {
	t.Parallel()
	object := NewString()
	object.AddProperty("invalid", NewString())
	_, err := object.Build(t.Context())
	require.Error(t, err)
	var buildErr *sdkerrors.BuildError
	require.True(t, errors.As(err, &buildErr))
	assert.Contains(t, buildErr.Error(), "properties can only be added to object schemas")
}

func TestBuilder_ValidateSchema(t *testing.T) {
	t.Parallel()
	object := NewObject().
		AddProperty("id", NewString().WithPattern(`^[a-z-]+$`)).
		RequireProperty("id")
	require.NoError(t, object.ValidateSchema(t.Context()))
}

func TestBuilder_TestAgainstSample(t *testing.T) {
	t.Parallel()
	object := NewObject().
		AddProperty("answer", NewString().WithMinLength(1)).
		AddProperty("confidence", NewNumber().WithMinimum(0).WithMaximum(1)).
		RequireProperty("answer")
	sample := map[string]any{
		"answer":     "42",
		"confidence": 0.99,
	}
	require.NoError(t, object.TestAgainstSample(t.Context(), sample))
	invalid := map[string]any{"confidence": 2}
	err := object.TestAgainstSample(t.Context(), invalid)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema validation failed")
}

func TestBuilder_NestedSchemas(t *testing.T) {
	t.Parallel()
	address := NewObject().
		AddProperty("street", NewString().WithMinLength(3)).
		AddProperty("zip", NewString().WithPattern(`^\d{5}$`)).
		RequireProperty("street").
		RequireProperty("zip")
	user := NewObject().
		AddProperty("name", NewString().WithMinLength(1)).
		AddProperty("address", address).
		RequireProperty("name").
		RequireProperty("address")
	require.NoError(t, user.ValidateSchema(t.Context()))
}

func TestBuilder_ArrayOfObjects(t *testing.T) {
	t.Parallel()
	item := NewObject().
		AddProperty("id", NewString().WithMinLength(1)).
		AddProperty("value", NewNumber()).
		RequireProperty("id")
	array := NewArray(item).WithMinItems(1)
	require.NoError(t, array.ValidateSchema(t.Context()))
}

func TestBuilder_WithRef(t *testing.T) {
	t.Parallel()
	s := NewString().WithRef("#/components/schemas/User")
	result, err := s.Build(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "#/components/schemas/User", (*result)["$ref"])
	_, exists := (*result)["type"]
	assert.False(t, exists)
}

func TestBuilder_BuildErrorAggregation(t *testing.T) {
	t.Parallel()
	builder := NewString().WithMinLength(5).WithMaxLength(1)
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	var buildErr *sdkerrors.BuildError
	require.True(t, errors.As(err, &buildErr))
	assert.Contains(t, buildErr.Error(), "minLength cannot exceed maxLength")
}

func TestBuilder_TestAgainstSampleNilSchema(t *testing.T) {
	t.Parallel()
	var builder *Builder
	_, err := builder.Build(t.Context())
	require.Error(t, err)
}

func TestBuilder_BuildContextRequired(t *testing.T) {
	t.Parallel()
	builder := NewString()
	_, err := builder.Build(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context is required")
}

func TestBuilder_TestAgainstSampleWithoutProperties(t *testing.T) {
	t.Parallel()
	object := NewObject().RequireProperty("missing")
	err := object.TestAgainstSample(t.Context(), map[string]any{"missing": "value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required property \"missing\" is not defined")
}

func TestBuilder_BuildClonesSchema(t *testing.T) {
	t.Parallel()
	builder := NewObject().AddProperty("name", NewString())
	result, err := builder.Build(t.Context())
	require.NoError(t, err)
	props := (*result)["properties"].(map[string]any)
	props["name"].(map[string]any)["type"] = "integer"
	second, err := builder.Build(t.Context())
	require.NoError(t, err)
	props2 := (*second)["properties"].(map[string]any)
	assert.Equal(t, typeString, props2["name"].(map[string]any)["type"])
}

func BenchmarkBuilderValidateSchema(b *testing.B) {
	builder := NewObject().
		AddProperty("id", NewString().WithPattern(`^[a-z-]+$`)).
		AddProperty("score", NewNumber().WithMinimum(0).WithMaximum(100)).
		AddProperty("tags", NewArray(NewString()).WithMinItems(0)).
		RequireProperty("id")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := builder.ValidateSchema(b.Context()); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestBuilder_ValidateSchemaWithReference(t *testing.T) {
	t.Parallel()
	builder := NewString().WithRef("#/components/schemas/External")
	require.NoError(t, builder.ValidateSchema(t.Context()))
}

func TestBuilder_ArrayRequiresItems(t *testing.T) {
	t.Parallel()
	array := NewArray(nil)
	err := array.ValidateSchema(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "array schema must define items")
}

func TestBuilder_TestAgainstSampleNilSchemaValue(t *testing.T) {
	t.Parallel()
	var sch *schema.Schema
	result, err := sch.Validate(t.Context(), map[string]any{})
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestBuilder_AddPropertyNilProperty(t *testing.T) {
	t.Parallel()
	object := NewObject().AddProperty("name", nil)
	_, err := object.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestBuilder_AddPropertyEmptyName(t *testing.T) {
	t.Parallel()
	object := NewObject().AddProperty(" ", NewString())
	_, err := object.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "property name cannot be empty")
}

func TestBuilder_RequirePropertyDuplicate(t *testing.T) {
	t.Parallel()
	object := NewObject().
		AddProperty("name", NewString()).
		RequireProperty("name").
		RequireProperty("name")
	result, err := object.Build(t.Context())
	require.NoError(t, err)
	required := (*result)["required"].([]string)
	assert.Equal(t, []string{"name"}, required)
}

func TestBuilder_StringConstraintWrongType(t *testing.T) {
	t.Parallel()
	builder := NewNumber().
		WithMinLength(1).
		WithPattern(".*").
		WithEnum("a")
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for string schemas")
}

func TestBuilder_NumberConstraintWrongType(t *testing.T) {
	t.Parallel()
	builder := NewString().WithMinimum(1)
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for number or integer schemas")
}

func TestBuilder_ArrayConstraintWrongType(t *testing.T) {
	t.Parallel()
	builder := NewString().WithMinItems(1)
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for array schemas")
}

func TestBuilder_WithItemsNilError(t *testing.T) {
	t.Parallel()
	array := NewArray(nil).WithItems(nil)
	_, err := array.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "array item schema cannot be nil")
}

func TestBuilder_WithDescriptionEmpty(t *testing.T) {
	t.Parallel()
	builder := NewString().WithDescription(" ")
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description cannot be empty")
}

func TestBuilder_WithRefEmpty(t *testing.T) {
	t.Parallel()
	builder := NewString().WithRef(" ")
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema reference ID cannot be empty")
}

func TestBuilder_WithRefRemovesTypeSpecificKeys(t *testing.T) {
	t.Parallel()
	builder := NewString().WithMinLength(1).WithRef("#/ref")
	result, err := builder.Build(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "#/ref", (*result)["$ref"])
	_, hasMinLength := (*result)["minLength"]
	assert.False(t, hasMinLength)
}

func TestBuilder_ArrayMissingItemsError(t *testing.T) {
	t.Parallel()
	array := NewArray(nil)
	_, err := array.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "array schema must define items")
}

func TestBuilder_CompileError(t *testing.T) {
	t.Parallel()
	invalid := &Builder{
		schema: schema.Schema{
			"type": typeObject,
			"properties": map[string]any{
				"broken": map[string]any{"type": 123},
			},
		},
		errors: make([]error, 0),
	}
	err := invalid.ValidateSchema(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema compilation failed")
}

func TestBuilder_WithMinLengthNegative(t *testing.T) {
	t.Parallel()
	builder := NewString().WithMinLength(-1)
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minLength must be non-negative")
}

func TestBuilder_WithMaxLengthNegative(t *testing.T) {
	t.Parallel()
	builder := NewString().WithMaxLength(-1)
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maxLength must be non-negative")
}

func TestBuilder_WithPatternInvalid(t *testing.T) {
	t.Parallel()
	builder := NewString().WithPattern("[")
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pattern")
}

func TestBuilder_WithEnumEmpty(t *testing.T) {
	t.Parallel()
	builder := NewString().WithEnum()
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enum requires at least one value")
}

func TestBuilder_WithMaximumWrongType(t *testing.T) {
	t.Parallel()
	builder := NewString().WithMaximum(5)
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for number or integer schemas")
}

func TestBuilder_WithMaxItemsNegative(t *testing.T) {
	t.Parallel()
	array := NewArray(NewString()).WithMaxItems(-1)
	_, err := array.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be non-negative")
}

func TestBuilder_WithItemsWrongType(t *testing.T) {
	t.Parallel()
	builder := NewString().WithItems(NewString())
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "items can only be configured for array schemas")
}

func TestBuilder_WithItemsPropagatesErrors(t *testing.T) {
	t.Parallel()
	item := NewString().WithMinLength(-1)
	array := NewArray(NewString()).WithItems(item)
	_, err := array.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minLength must be non-negative")
}

func TestBuilder_NumberBoundsValidation(t *testing.T) {
	t.Parallel()
	builder := NewNumber().WithMinimum(10).WithMaximum(1)
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minimum cannot exceed maximum")
}

func TestBuilder_ArrayBoundsValidation(t *testing.T) {
	t.Parallel()
	array := NewArray(NewString()).
		WithMinItems(5).
		WithMaxItems(1)
	_, err := array.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minItems cannot exceed maxItems")
}

func TestBuilder_EnsurePropertiesNonMap(t *testing.T) {
	t.Parallel()
	custom := &Builder{
		schema: schema.Schema{
			"type":       typeObject,
			"properties": []int{1, 2},
		},
		errors: make([]error, 0),
	}
	custom.AddProperty("field", NewString())
	_, err := custom.Build(t.Context())
	require.NoError(t, err)
}

func TestBuilder_NilReceiverFluentMethods(t *testing.T) {
	t.Parallel()
	var builder *Builder
	assert.Nil(t, builder.AddProperty("field", NewString()))
	assert.Nil(t, builder.RequireProperty("field"))
	assert.Nil(t, builder.WithMinLength(1))
	assert.Nil(t, builder.WithMaxLength(1))
	assert.Nil(t, builder.WithPattern(".*"))
	assert.Nil(t, builder.WithEnum("x"))
	assert.Nil(t, builder.WithMinimum(1))
	assert.Nil(t, builder.WithMaximum(1))
	assert.Nil(t, builder.WithMinItems(1))
	assert.Nil(t, builder.WithMaxItems(1))
	assert.Nil(t, builder.WithItems(NewString()))
	assert.Nil(t, builder.WithDefault("value"))
	assert.Nil(t, builder.WithDescription("desc"))
	assert.Nil(t, builder.WithRef("#/ref"))
}

func TestBuilder_NilReceiverValidationMethods(t *testing.T) {
	t.Parallel()
	var builder *Builder
	err := builder.ValidateSchema(t.Context())
	require.Error(t, err)
	err = builder.TestAgainstSample(t.Context(), map[string]any{})
	require.Error(t, err)
}

func TestBuilder_RequirePropertyWrongType(t *testing.T) {
	t.Parallel()
	builder := NewString().RequireProperty("field")
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only be set on object schemas")
}

func TestBuilder_RequirePropertyEmptyName(t *testing.T) {
	t.Parallel()
	builder := NewObject().RequireProperty(" ")
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required property name cannot be empty")
}

func TestBuilder_WithMaxLengthWrongType(t *testing.T) {
	t.Parallel()
	builder := NewNumber().WithMaxLength(5)
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for string schemas")
}

func TestBuilder_WithEnumEmptyString(t *testing.T) {
	t.Parallel()
	builder := NewString().WithEnum("valid", " ")
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enum values cannot be empty")
}

func TestBuilder_WithMinItemsNegative(t *testing.T) {
	t.Parallel()
	array := NewArray(NewString()).WithMinItems(-1)
	_, err := array.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minItems must be non-negative")
}

func TestBuilder_WithMaxItemsWrongType(t *testing.T) {
	t.Parallel()
	builder := NewString().WithMaxItems(1)
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for array schemas")
}

func TestBuilder_ValidateSchemaUnsupportedType(t *testing.T) {
	t.Parallel()
	builder := &Builder{
		schema: schema.Schema{"type": "unknown"},
		errors: make([]error, 0),
	}
	err := builder.ValidateSchema(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported schema type")
}

func TestBuilder_TestAgainstSampleUnsupportedType(t *testing.T) {
	t.Parallel()
	builder := &Builder{
		schema: schema.Schema{"type": "unknown"},
		errors: make([]error, 0),
	}
	err := builder.TestAgainstSample(t.Context(), map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported schema type")
}

func TestBuilder_ValidateObjectRequiredNotSlice(t *testing.T) {
	t.Parallel()
	builder := &Builder{
		schema: schema.Schema{
			"type":       typeObject,
			"properties": map[string]any{},
			"required":   map[string]any{},
		},
		errors: make([]error, 0),
	}
	_, err := builder.Build(t.Context())
	require.NoError(t, err)
}

func TestBuilder_IsNumericNonString(t *testing.T) {
	t.Parallel()
	builder := &Builder{
		schema: schema.Schema{"type": 123},
		errors: make([]error, 0),
	}
	assert.False(t, builder.isNumeric())
}
