package spec

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

func describeTaskBlockedReasonsProperty(schema *openapi3.Schema) {
	if schema == nil || schema.Properties == nil {
		return
	}
	property := schema.Properties["blocked_reasons"]
	if property == nil || property.Value == nil {
		return
	}
	property.Value.Description = "Read projection of current blocking causes; mutation responses may omit it."
}

func customizeSchedulerDrainRequestSchema(schema *openapi3.Schema) {
	if schema == nil || schema.Properties == nil {
		return
	}
	timeoutSeconds := schema.Properties["timeout_seconds"]
	if timeoutSeconds == nil || timeoutSeconds.Value == nil {
		return
	}
	timeoutSeconds.Value.
		WithMin(0).
		WithMax(float64(contract.SchedulerDrainTimeoutMaxSeconds))
}
