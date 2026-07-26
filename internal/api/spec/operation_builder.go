package spec

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

func buildOperation(schemas openapi3.Schemas, spec OperationSpec) (*openapi3.Operation, error) {
	operation := openapi3.NewOperation()
	operation.OperationID = spec.OperationID
	operation.Summary = spec.Summary
	operation.Tags = append([]string(nil), spec.Tags...)
	operation.Extensions = map[string]any{"x-agh-transports": spec.Transports}

	for _, param := range spec.Parameters {
		operation.AddParameter(buildParameter(param))
	}
	if spec.RequestBody != nil {
		schemaRef, err := schemaRefForValue(spec.RequestBody, schemas)
		if err != nil {
			return nil, fmt.Errorf("request body schema: %w", err)
		}
		operation.RequestBody = &openapi3.RequestBodyRef{
			Value: openapi3.NewRequestBody().
				WithContent(openapi3.NewContentWithJSONSchemaRef(schemaRef)).
				WithDescription("JSON request body"),
		}
		operation.RequestBody.Value.Required = !spec.RequestBodyOptional
	}
	for _, response := range spec.Responses {
		built, err := buildResponse(schemas, response)
		if err != nil {
			return nil, err
		}
		operation.AddResponse(response.Status, built)
	}
	return operation, nil
}

func buildResponse(schemas openapi3.Schemas, response ResponseSpec) (*openapi3.Response, error) {
	bodies := response.Bodies
	if response.Body != nil {
		if len(bodies) > 0 {
			return nil, fmt.Errorf("response %d cannot define Body and Bodies", response.Status)
		}
		bodies = []any{response.Body}
	}
	result := openapi3.NewResponse().WithDescription(response.Description)
	if len(bodies) == 0 {
		return result, nil
	}
	schemaRefs := make([]*openapi3.SchemaRef, 0, len(bodies))
	for _, body := range bodies {
		schemaRef, err := schemaRefForValue(body, schemas)
		if err != nil {
			return nil, fmt.Errorf("response %d schema: %w", response.Status, err)
		}
		schemaRefs = append(schemaRefs, schemaRef)
	}
	schemaRef := schemaRefs[0]
	if len(schemaRefs) > 1 {
		schemaRef = &openapi3.SchemaRef{Value: &openapi3.Schema{OneOf: schemaRefs}}
	}
	contentType := response.ContentType
	if contentType == "" {
		contentType = "application/json"
	}
	result.WithContent(openapi3.Content{contentType: &openapi3.MediaType{Schema: schemaRef}})
	return result, nil
}
