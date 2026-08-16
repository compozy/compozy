package spec

import (
	"github.com/compozy/compozy/internal/session"
	"github.com/getkin/kin-openapi/openapi3"
)

func customizeAgentNotifyRequestSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	setNotificationTextCap(schema, "title", session.NotificationTitleMaxChars, true)
	setNotificationTextCap(schema, "body", session.NotificationBodyMaxChars, false)
}

func setNotificationTextCap(schema *openapi3.Schema, field string, max int, required bool) {
	if schema == nil {
		return
	}
	property, exists := schema.Properties[field]
	if !exists || property == nil || property.Value == nil {
		return
	}
	maxLength := uint64(max)
	property.Value.MaxLength = &maxLength
	if required {
		property.Value.MinLength = 1
	}
}
