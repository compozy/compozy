package spec

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

func customizeAttachSessionRequestSchema(schema *openapi3.Schema) {
	if schema == nil || schema.Properties == nil {
		return
	}
	ttlSeconds := schema.Properties["ttl_seconds"]
	if ttlSeconds == nil || ttlSeconds.Value == nil {
		return
	}
	ttlSeconds.Value.WithMax(float64(contract.SessionAttachMaxTTLSeconds))
}
