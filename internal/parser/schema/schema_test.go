package schema

import (
	"testing"

	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/stretchr/testify/assert"
)

func TestSchemaValidator(t *testing.T) {
	tests := []struct {
		name         string
		pkgRef       *pkgref.PackageRefConfig
		inputSchema  *InputSchema
		outputSchema *OutputSchema
		wantErr      bool
		errMsg       string
	}{
		{
			name: "Valid top-level object schema",
			inputSchema: &InputSchema{
				Schema: Schema{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid top-level non-object schema",
			inputSchema: &InputSchema{
				Schema: Schema{
					"type": "string",
				},
			},
			wantErr: true,
			errMsg:  ErrMsgInvalidSchemaType,
		},
		{
			name: "Invalid top-level object without properties",
			inputSchema: &InputSchema{
				Schema: Schema{
					"type": "object",
				},
			},
			wantErr: true,
			errMsg:  ErrMsgMissingSchemaProps,
		},
		{
			name: "Valid nested non-object schema",
			inputSchema: &InputSchema{
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
			},
			wantErr: false,
		},
		{
			name: "Valid nested array schema",
			inputSchema: &InputSchema{
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
			},
			wantErr: false,
		},
		{
			name: "Valid nested object schema",
			inputSchema: &InputSchema{
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
			},
			wantErr: false,
		},
		{
			name: "Valid schema with composition",
			inputSchema: &InputSchema{
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
			},
			wantErr: false,
		},
		{
			name:   "Invalid package reference",
			pkgRef: pkgref.NewPackageRefConfig("invalid"),
			inputSchema: &InputSchema{
				Schema: Schema{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "Invalid package reference",
		},
		{
			name:   "Input schema not allowed with ID reference",
			pkgRef: pkgref.NewPackageRefConfig("agent(id=test-agent)"),
			inputSchema: &InputSchema{
				Schema: Schema{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "Input schema not allowed for reference type id",
		},
		{
			name:   "Output schema not allowed with file reference",
			pkgRef: pkgref.NewPackageRefConfig("agent(file=test.yaml)"),
			outputSchema: &OutputSchema{
				Schema: Schema{
					"type": "object",
					"properties": map[string]any{
						"result": map[string]any{
							"type": "string",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "Output schema not allowed for reference type file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewSchemaValidator(tt.pkgRef, tt.inputSchema, tt.outputSchema)
			err := validator.Validate()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidate(t *testing.T) {
	tests := []struct {
		name    string
		schema  *Schema
		value   any
		wantErr bool
	}{
		{
			name: "Valid string",
			schema: &Schema{
				"type": "string",
			},
			value:   "test",
			wantErr: false,
		},
		{
			name: "Invalid string",
			schema: &Schema{
				"type": "string",
			},
			value:   123,
			wantErr: true,
		},
		{
			name: "Valid number",
			schema: &Schema{
				"type": "number",
			},
			value:   123.45,
			wantErr: false,
		},
		{
			name: "Invalid number",
			schema: &Schema{
				"type": "number",
			},
			value:   "test",
			wantErr: true,
		},
		{
			name: "Valid object",
			schema: &Schema{
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
			},
			value: map[string]any{
				"name": "John",
				"age":  30,
			},
			wantErr: false,
		},
		{
			name: "Invalid object - missing required",
			schema: &Schema{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
				},
				"required": []string{"name"},
			},
			value:   map[string]any{},
			wantErr: true,
		},
		{
			name: "Invalid object - wrong type",
			schema: &Schema{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
				},
			},
			value: map[string]any{
				"name": 123,
			},
			wantErr: true,
		},
		{
			name: "Valid array",
			schema: &Schema{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			},
			value:   []any{"a", "b", "c"},
			wantErr: false,
		},
		{
			name: "Invalid array item type",
			schema: &Schema{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			},
			value:   []any{"a", 2, "c"},
			wantErr: true,
		},
		{
			name:    "Nil schema",
			schema:  nil,
			value:   "test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
