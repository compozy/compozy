package schema

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/pkg/ref"
	"github.com/pkg/errors"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// -----------------------------------------------------------------------------
// Schema
// -----------------------------------------------------------------------------

// Schema represents a JSON schema
type Schema map[string]any

type InputSchema struct {
	ref.WithRef
	Ref    any    `json:"$ref,omitempty" yaml:"$ref,omitempty" is_ref:"true"`
	Schema Schema `yaml:",inline"`
}

func (s *InputSchema) ResolveRef(ctx context.Context, currentDoc any, projectRoot, filePath string) error {
	// Handle the case where we have a Ref field that needs to be merged with existing Schema
	if err := resolveSchemaRef(ctx, &s.WithRef, s.Ref, &s.Schema, currentDoc, projectRoot, filePath, "input"); err != nil {
		return err
	}
	// Then resolve any nested $ref fields within the schema itself
	if err := resolveNestedSchemaRefs(ctx, &s.WithRef, &s.Schema, currentDoc, projectRoot, filePath); err != nil {
		return errors.Wrap(err, "failed to resolve nested input schema references")
	}
	return nil
}

type OutputSchema struct {
	ref.WithRef
	Ref    any    `json:"$ref,omitempty" yaml:"$ref,omitempty" is_ref:"true"`
	Schema Schema `yaml:",inline"`
}

func (s *OutputSchema) ResolveRef(ctx context.Context, currentDoc any, projectRoot, filePath string) error {
	// Handle the case where we have a Ref field that needs to be merged with existing Schema
	if err := resolveSchemaRef(ctx, &s.WithRef, s.Ref, &s.Schema, currentDoc, projectRoot, filePath, "output"); err != nil {
		return err
	}
	// Then resolve any nested $ref fields within the schema itself
	if err := resolveNestedSchemaRefs(ctx, &s.WithRef, &s.Schema, currentDoc, projectRoot, filePath); err != nil {
		return errors.Wrap(err, "failed to resolve nested output schema references")
	}
	return nil
}

// resolveSchemaRef handles ref resolution for both InputSchema and OutputSchema
func resolveSchemaRef(ctx context.Context, withRef *ref.WithRef, refField any, schema *Schema, currentDoc any, projectRoot, filePath, schemaType string) error {
	withRef.SetRefMetadata(filePath, projectRoot)
	resolvedMap, err := withRef.ResolveRefWithInlineData(ctx, refField, map[string]any(*schema), currentDoc)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve %s schema $ref", schemaType)
	}
	// Update the schema with the resolved result
	*schema = Schema(resolvedMap)
	return nil
}

// resolveNestedSchemaRefs resolves any $ref fields within the schema itself
func resolveNestedSchemaRefs(ctx context.Context, withRef *ref.WithRef, schema *Schema, currentDoc any, projectRoot, filePath string) error {
	if *schema == nil {
		return nil
	}
	// Check if the schema contains $ref fields and resolve them
	if _, hasRef := (*schema)["$ref"]; hasRef {
		// Ensure the WithRef metadata is properly set
		withRef.SetRefMetadata(filePath, projectRoot)
		resolvedMap, err := withRef.ResolveMapReference(ctx, map[string]any(*schema), currentDoc)
		if err != nil {
			return err
		}
		*schema = Schema(resolvedMap)
	}
	return nil
}

func (s *Schema) Validate(value any) error {
	if s == nil {
		return nil
	}
	// Convert schema to JSON for jsonschema
	schemaJSON, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}
	// Compile the schema
	schema, err := jsonschema.CompileString("schema.json", string(schemaJSON))
	if err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}
	// Perform validation
	if err := schema.Validate(value); err != nil {
		return err
	}
	return nil
}

func (s *Schema) GetType() string {
	if typ, ok := (*s)["type"].(string); ok {
		return typ
	}
	return ""
}

func (s *Schema) GetProperties() map[string]*Schema {
	props, ok := (*s)["properties"].(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]*Schema)
	for k, v := range props {
		if schema, ok := v.(map[string]any); ok {
			s := Schema(schema)
			result[k] = &s
		}
	}
	return result
}

// SchemaContainer defines an interface for structs that contain input and output schemas
type SchemaContainer interface {
	GetInputSchema() *InputSchema
	SetInputSchema(*InputSchema)
	GetOutputSchema() *OutputSchema
	SetOutputSchema(*OutputSchema)
}

// ResolveConfigSchemas handles complete schema resolution for config structs.
// This consolidates both top-level reference resolution and individual schema resolution.
func ResolveConfigSchemas(
	ctx context.Context,
	withRef *ref.WithRef,
	refField any,
	target any,
	currentDoc map[string]any,
	projectRoot, filePath string,
	schemaContainer SchemaContainer,
) error {
	// Resolve top-level reference if present
	if refField != nil {
		withRef.SetRefMetadata(filePath, projectRoot)
		if hasRef := isValidRef(refField); hasRef {
			if err := withRef.ResolveAndMergeReferences(
				ctx,
				target,
				currentDoc,
				ref.ModeMerge,
			); err != nil {
				return errors.Wrap(err, "failed to resolve top-level reference")
			}
			// Process schemas from resolved reference data
			if err := processSchemaFromResolvedRef(
				ctx, withRef, refField, currentDoc, projectRoot, filePath, schemaContainer,
			); err != nil {
				return errors.Wrap(err, "failed to process schemas from resolved reference")
			}
		}
	}
	// Resolve individual schema references
	if inputSchema := schemaContainer.GetInputSchema(); inputSchema != nil {
		if err := inputSchema.ResolveRef(ctx, currentDoc, projectRoot, filePath); err != nil {
			return errors.Wrap(err, "failed to resolve input schema reference")
		}
	}
	if outputSchema := schemaContainer.GetOutputSchema(); outputSchema != nil {
		if err := outputSchema.ResolveRef(ctx, currentDoc, projectRoot, filePath); err != nil {
			return errors.Wrap(err, "failed to resolve output schema reference")
		}
	}
	return nil
}

// processSchemaFromResolvedRef handles the case where schemas need to be populated from resolved reference data
func processSchemaFromResolvedRef(
	ctx context.Context,
	withRef *ref.WithRef,
	refField any,
	currentDoc map[string]any,
	projectRoot, filePath string,
	schemaContainer SchemaContainer,
) error {
	parsedRef, err := withRef.ParseRefFromValue(refField)
	if err != nil {
		return errors.Wrap(err, "failed to parse reference")
	}
	if parsedRef == nil {
		return nil
	}
	resolvedData, err := parsedRef.Resolve(ctx, currentDoc, filePath, projectRoot)
	if err != nil {
		return errors.Wrap(err, "failed to resolve reference")
	}
	if resolvedMap, ok := resolvedData.(map[string]any); ok {
		// Process input schema
		if inputData, exists := resolvedMap["input"]; exists && schemaContainer.GetInputSchema() == nil {
			if inputMap, ok := inputData.(map[string]any); ok {
				schemaContainer.SetInputSchema(&InputSchema{
					Schema: Schema(inputMap),
				})
			}
		}
		// Process output schema
		if outputData, exists := resolvedMap["output"]; exists && schemaContainer.GetOutputSchema() == nil {
			if outputMap, ok := outputData.(map[string]any); ok {
				schemaContainer.SetOutputSchema(&OutputSchema{
					Schema: Schema(outputMap),
				})
			}
		}
	}
	return nil
}

// isValidRef checks if a reference field contains a valid reference
func isValidRef(refField any) bool {
	switch v := refField.(type) {
	case string:
		return v != ""
	case map[string]any:
		return len(v) > 0
	default:
		return false
	}
}
