package spec

import (
	networkpkg "github.com/compozy/agh/internal/network"
	"github.com/getkin/kin-openapi/openapi3"
)

func customizeNetworkSendRequestSchema(schema *openapi3.Schema) {
	if schema == nil {
		return
	}
	schema.WithoutAdditionalProperties()
	schema.Description = "For say, capability, receipt, and trace, surface is required. " +
		"surface=thread requires thread_id; the first valid send creates the public thread. " +
		"surface=direct requires an existing direct_id. " +
		"capability, receipt, and trace also require work_id. " +
		"capability and lifecycle-bearing say also require to. " +
		"greet and whois omit conversation and work fields."

	kind := networkSendProperty(schema, "kind")
	if kind != nil {
		kind.Enum = enumAsAny([]string{
			string(networkpkg.KindGreet),
			string(networkpkg.KindWhois),
			string(networkpkg.KindSay),
			string(networkpkg.KindCapability),
			string(networkpkg.KindReceipt),
			string(networkpkg.KindTrace),
		})
	}
	surface := networkSendProperty(schema, "surface")
	if surface != nil {
		surface.Enum = enumAsAny([]string{string(networkpkg.SurfaceThread), string(networkpkg.SurfaceDirect)})
		surface.Description = "Required for say, capability, receipt, and trace; omitted for greet and whois."
	}
	threadID := networkSendProperty(schema, "thread_id")
	if threadID != nil {
		threadID.Pattern = networkpkg.ThreadIDPattern
		threadID.Description = "Required with surface=thread. Use a fresh valid ID. " +
			"The first valid send creates the public thread; later sends reuse the same ID."
	}
	directID := networkSendProperty(schema, "direct_id")
	if directID != nil {
		directID.Pattern = networkpkg.DirectIDPattern
		directID.Description = "Required with surface=direct. Resolve the existing room before sending."
	}
	workID := networkSendProperty(schema, "work_id")
	if workID != nil {
		workID.Description = "Required for capability, receipt, and trace; " +
			"optional for lifecycle-bearing say messages."
	}
	target := networkSendProperty(schema, "to")
	if target != nil {
		target.Description = "Target peer. Required for capability and for say carrying work_id."
	}
	body := networkSendProperty(schema, "body")
	if body != nil {
		*body = *openapi3.NewObjectSchema().WithAdditionalProperties(openapi3.NewSchema())
		body.Description = "Kind-specific object. say requires a non-empty text field; " +
			"other kinds follow the AGH Network message body rules."
	}
}

func networkSendProperty(schema *openapi3.Schema, name string) *openapi3.Schema {
	if schema == nil || schema.Properties == nil {
		return nil
	}
	property := schema.Properties[name]
	if property == nil {
		return nil
	}
	return property.Value
}
