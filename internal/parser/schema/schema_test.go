package schema

import (
	"testing"

	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/stretchr/testify/assert"
)

func Test_SchemaValidator(t *testing.T) {
	t.Run("Should validate valid top-level object schema", func(t *testing.T) {
		inputSchema := &InputSchema{
			Schema: Schema{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
				},
			},
		}

		validator := NewSchemaValidator(nil, inputSchema, nil)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error for invalid top-level non-object schema", func(t *testing.T) {
		inputSchema := &InputSchema{
			Schema: Schema{
				"type": "string",
			},
		}

		validator := NewSchemaValidator(nil, inputSchema, nil)
		err := validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ErrMsgInvalidSchemaType)
	})

	t.Run("Should return error for invalid top-level object without properties", func(t *testing.T) {
		inputSchema := &InputSchema{
			Schema: Schema{
				"type": "object",
			},
		}

		validator := NewSchemaValidator(nil, inputSchema, nil)
		err := validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ErrMsgMissingSchemaProps)
	})

	t.Run("Should validate valid nested non-object schema", func(t *testing.T) {
		inputSchema := &InputSchema{
			Schema: Schema{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
					"age": map[string]any{
						"type": "number",
					},
					"isActive": map[string]any{
						"type": "boolean",
					},
				},
			},
		}

		validator := NewSchemaValidator(nil, inputSchema, nil)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should validate valid nested array schema", func(t *testing.T) {
		inputSchema := &InputSchema{
			Schema: Schema{
				"type": "object",
				"properties": map[string]any{
					"tags": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
					},
				},
			},
		}

		validator := NewSchemaValidator(nil, inputSchema, nil)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should validate valid nested object schema", func(t *testing.T) {
		inputSchema := &InputSchema{
			Schema: Schema{
				"type": "object",
				"properties": map[string]any{
					"address": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"street": map[string]any{
								"type": "string",
							},
							"city": map[string]any{
								"type": "string",
							},
						},
					},
				},
			},
		}

		validator := NewSchemaValidator(nil, inputSchema, nil)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should validate valid schema with composition", func(t *testing.T) {
		inputSchema := &InputSchema{
			Schema: Schema{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{
						"anyOf": []any{
							map[string]any{
								"type": "string",
								"enum": []any{"active", "inactive"},
							},
							map[string]any{
								"type": "boolean",
							},
						},
					},
				},
			},
		}

		validator := NewSchemaValidator(nil, inputSchema, nil)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error for invalid package reference", func(t *testing.T) {
		pkgRef := pkgref.NewPackageRefConfig("invalid")
		inputSchema := &InputSchema{
			Schema: Schema{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
				},
			},
		}

		validator := NewSchemaValidator(pkgRef, inputSchema, nil)
		err := validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid package reference")
	})

	t.Run("Should return error when input schema is used with ID reference", func(t *testing.T) {
		pkgRef := pkgref.NewPackageRefConfig("agent(id=test-agent)")
		inputSchema := &InputSchema{
			Schema: Schema{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
				},
			},
		}

		validator := NewSchemaValidator(pkgRef, inputSchema, nil)
		err := validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Input schema not allowed for reference type id")
	})

	t.Run("Should return error when output schema is used with file reference", func(t *testing.T) {
		pkgRef := pkgref.NewPackageRefConfig("agent(file=test.yaml)")
		outputSchema := &OutputSchema{
			Schema: Schema{
				"type": "object",
				"properties": map[string]any{
					"result": map[string]any{
						"type": "string",
					},
				},
			},
		}

		validator := NewSchemaValidator(pkgRef, nil, outputSchema)
		err := validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Output schema not allowed for reference type file")
	})
}

func Test_SchemaValidate(t *testing.T) {
	t.Run("Should validate valid string value", func(t *testing.T) {
		schema := &Schema{
			"type": "string",
		}
		err := schema.Validate("test")
		assert.NoError(t, err)
	})

	t.Run("Should return error for invalid string value", func(t *testing.T) {
		schema := &Schema{
			"type": "string",
		}
		err := schema.Validate(123)
		assert.Error(t, err)
	})

	t.Run("Should validate valid number value", func(t *testing.T) {
		schema := &Schema{
			"type": "number",
		}
		err := schema.Validate(123.45)
		assert.NoError(t, err)
	})

	t.Run("Should return error for invalid number value", func(t *testing.T) {
		schema := &Schema{
			"type": "number",
		}
		err := schema.Validate("test")
		assert.Error(t, err)
	})

	t.Run("Should validate valid object value", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
				"age": map[string]any{
					"type": "number",
				},
			},
			"required": []string{"name"},
		}
		value := map[string]any{
			"name": "John",
			"age":  30,
		}
		err := schema.Validate(value)
		assert.NoError(t, err)
	})

	t.Run("Should return error for object missing required field", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
			},
			"required": []string{"name"},
		}
		err := schema.Validate(map[string]any{})
		assert.Error(t, err)
	})

	t.Run("Should return error for object with wrong field type", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
			},
		}
		value := map[string]any{
			"name": 123,
		}
		err := schema.Validate(value)
		assert.Error(t, err)
	})

	t.Run("Should validate valid array value", func(t *testing.T) {
		schema := &Schema{
			"type": "array",
			"items": map[string]any{
				"type": "string",
			},
		}
		value := []any{"a", "b", "c"}
		err := schema.Validate(value)
		assert.NoError(t, err)
	})

	t.Run("Should return error for array with invalid item type", func(t *testing.T) {
		schema := &Schema{
			"type": "array",
			"items": map[string]any{
				"type": "string",
			},
		}
		value := []any{"a", 2, "c"}
		err := schema.Validate(value)
		assert.Error(t, err)
	})

	t.Run("Should handle nil schema", func(t *testing.T) {
		var schema *Schema
		err := schema.Validate("test")
		assert.NoError(t, err)
	})
}
