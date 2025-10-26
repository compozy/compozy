package schema

import (
	"errors"
	"testing"

	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPropertyBuilder_BuildStringProperty(t *testing.T) {
	t.Parallel()
	property, err := NewProperty("title").
		WithType("string").
		WithMinLength(3).
		WithMaxLength(30).
		WithPattern(`^[A-Z][A-Za-z0-9\s]+$`).
		WithDescription("Title of the document").
		WithDefault("Draft").
		Required().
		Build(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "title", property.Name)
	assert.True(t, property.Required)
	assert.Equal(t, typeString, (*property.Schema)["type"])
	assert.Equal(t, 3, (*property.Schema)["minLength"])
	assert.Equal(t, 30, (*property.Schema)["maxLength"])
	assert.Equal(t, "Draft", (*property.Schema)["default"])
	assert.Equal(t, "Title of the document", (*property.Schema)["description"])
}

func TestPropertyBuilder_ObjectProperty(t *testing.T) {
	t.Parallel()
	property, err := NewProperty("address").
		WithType("object").
		AddProperty("street", NewString().WithMinLength(3)).
		AddProperty("zip", NewString().WithPattern(`^\d{5}$`)).
		RequireProperty("street").
		RequireProperty("zip").
		Build(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "address", property.Name)
	props := (*property.Schema)["properties"].(map[string]any)
	assert.Contains(t, props, "street")
	assert.Contains(t, props, "zip")
	required := (*property.Schema)["required"].([]string)
	assert.ElementsMatch(t, []string{"street", "zip"}, required)
}

func TestPropertyBuilder_ArrayProperty(t *testing.T) {
	t.Parallel()
	item := NewObject().
		AddProperty("id", NewString().WithMinLength(1)).
		RequireProperty("id")
	property, err := NewProperty("items").
		WithType("array").
		WithItems(item).
		WithMinItems(1).
		WithMaxItems(10).
		Build(t.Context())
	require.NoError(t, err)
	assert.Equal(t, typeArray, (*property.Schema)["type"])
	items := (*property.Schema)["items"].(map[string]any)
	assert.Equal(t, typeObject, items["type"])
}

func TestPropertyBuilder_MissingType(t *testing.T) {
	t.Parallel()
	_, err := NewProperty("missing").Build(t.Context())
	require.Error(t, err)
	var buildErr *sdkerrors.BuildError
	require.True(t, errors.As(err, &buildErr))
	assert.Contains(t, buildErr.Error(), "type must be defined")
}

func TestPropertyBuilder_InvalidType(t *testing.T) {
	t.Parallel()
	_, err := NewProperty("invalid").WithType("unsupported").Build(t.Context())
	require.Error(t, err)
	var buildErr *sdkerrors.BuildError
	require.True(t, errors.As(err, &buildErr))
	assert.Contains(t, buildErr.Error(), "unsupported property type")
}

func TestPropertyBuilder_ForwardSchemaErrors(t *testing.T) {
	t.Parallel()
	_, err := NewProperty("username").
		WithType("string").
		WithMinLength(10).
		WithMaxLength(1).
		Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minLength cannot exceed maxLength")
}

func TestPropertyBuilder_NameValidation(t *testing.T) {
	t.Parallel()
	_, err := NewProperty(" ").WithType("string").Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "property name")
}

func TestPropertyBuilder_ContextRequired(t *testing.T) {
	t.Parallel()
	_, err := NewProperty("id").WithType("string").Build(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context is required")
}

func TestPropertyBuilder_BuildReturnsDeepCopy(t *testing.T) {
	t.Parallel()
	builder := NewProperty("status").WithType("string").WithEnum("active", "inactive")
	first, err := builder.Build(t.Context())
	require.NoError(t, err)
	enum := (*first.Schema)["enum"].([]string)
	enum[0] = "mutated"
	second, err := builder.Build(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"active", "inactive"}, (*second.Schema)["enum"])
}

func TestPropertyBuilder_DescriptionBeforeType(t *testing.T) {
	t.Parallel()
	builder := NewProperty("note")
	builder.WithDescription("description first")
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type must be defined")
}

func TestPropertyBuilder_DefaultBeforeType(t *testing.T) {
	t.Parallel()
	builder := NewProperty("value")
	builder.WithDefault("x")
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type must be defined")
}

func TestPropertyBuilder_AddPropertyBeforeType(t *testing.T) {
	t.Parallel()
	builder := NewProperty("details")
	builder.AddProperty("nested", NewString())
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type must be defined")
}

func TestPropertyBuilder_RequirePropertyBeforeType(t *testing.T) {
	t.Parallel()
	builder := NewProperty("details")
	builder.RequireProperty("nested")
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type must be defined")
}

func TestPropertyBuilder_WithItemsBeforeType(t *testing.T) {
	t.Parallel()
	builder := NewProperty("list")
	builder.WithItems(NewString())
	_, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type must be defined")
}

func TestPropertyBuilder_NumberConstraints(t *testing.T) {
	t.Parallel()
	property, err := NewProperty("score").
		WithType("number").
		WithMinimum(0).
		WithMaximum(100).
		Build(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 0.0, (*property.Schema)["minimum"])
	assert.Equal(t, 100.0, (*property.Schema)["maximum"])
}

func TestPropertyBuilder_NilReceiverMethods(t *testing.T) {
	t.Parallel()
	var builder *PropertyBuilder
	assert.Nil(t, builder.WithType("string"))
	assert.Nil(t, builder.WithDescription("desc"))
	assert.Nil(t, builder.WithDefault("value"))
	assert.Nil(t, builder.Required())
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
}

func TestPropertyBuilder_NilReceiverBuild(t *testing.T) {
	t.Parallel()
	var builder *PropertyBuilder
	_, err := builder.Build(t.Context())
	require.Error(t, err)
}
