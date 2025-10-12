package main

import (
	"encoding/json"
	"maps"
	"sync"

	"github.com/compozy/compozy/engine/attachment"
	"github.com/invopop/jsonschema"
)

const (
	schemaTypeObject      = "object"
	processorDescription  = "Nested processor task configuration (simplified to avoid infinite recursion in forms)."
	nestedTaskDescription = "Nested task configuration (simplified for UI forms)."
	taskItemDescription   = "Task item (simplified for UI forms)."
)

type attachmentsSchemaBundle struct {
	arrayDefinition    map[string]any
	variantDefinitions map[string]any
}

var (
	attachmentsSchemaOnce sync.Once
	attachmentsSchemaData attachmentsSchemaBundle
	attachmentsSchemaErr  error
)

func postProcessAgentSchema(schema map[string]any) bool {
	props, ok := mapValue(schema, "properties")
	if !ok {
		return false
	}
	updated := ensureArrayRef(props, "tools", "tool.json")

	if ensureArrayRef(props, "mcps", "mcp.json") {
		updated = true
	}
	if ensureArrayRef(props, "knowledge", "knowledge-binding.json") {
		updated = true
	}
	if ensureAttachmentsSchema(schema, props) {
		updated = true
	}
	if defs, ok := schema["$defs"].(map[string]any); ok {
		if actionDef, ok := mapValue(defs, "ActionConfig"); ok {
			if actionProps, ok := mapValue(actionDef, "properties"); ok {
				if ensureAttachmentsSchema(schema, actionProps) {
					updated = true
				}
			}
		}
	}
	return updated
}

func postProcessWorkflowSchema(schema map[string]any) bool {
	props, ok := mapValue(schema, "properties")
	if !ok {
		return false
	}
	updated := ensureArrayRef(props, "tools", "tool.json")

	if ensureArrayRef(props, "agents", "agent.json") {
		updated = true
	}
	if ensureArrayRef(props, "mcps", "mcp.json") {
		updated = true
	}
	if ensureArrayRef(props, "tasks", "task.json") {
		updated = true
	}
	if ensureArrayRef(props, "knowledge_bases", "knowledge-base.json") {
		updated = true
	}
	if ensureArrayRef(props, "knowledge", "knowledge-binding.json") {
		updated = true
	}
	if updateWorkflowTriggers(schema, props) {
		updated = true
	}
	return updated
}

func postProcessTaskSchema(schema map[string]any) bool {
	props, ok := mapValue(schema, "properties")
	if !ok {
		return false
	}
	updated := ensureRef(props, "agent", "agent.json")

	if ensureRef(props, "tool", "tool.json") {
		updated = true
	}
	if ensureArrayRef(props, "mcps", "mcp.json") {
		updated = true
	}
	if simplifyNestedTask(props, "processor", processorDescription) {
		updated = true
	}
	if simplifyNestedTask(props, "task", nestedTaskDescription) {
		updated = true
	}
	if simplifyNestedTaskArray(props, "tasks", taskItemDescription) {
		updated = true
	}
	if ensureArrayRef(props, "tools", "tool.json") {
		updated = true
	}
	if ensureAttachmentsSchema(schema, props) {
		updated = true
	}
	if ensureArrayRef(props, "knowledge", "knowledge-binding.json") {
		updated = true
	}
	return updated
}

func postProcessActionConfigSchema(schema map[string]any) bool {
	props, ok := mapValue(schema, "properties")
	if !ok {
		return false
	}
	return ensureAttachmentsSchema(schema, props)
}

func postProcessProjectSchema(schema map[string]any) bool {
	props, ok := mapValue(schema, "properties")
	if !ok {
		return false
	}
	updated := ensureRef(props, "cache", "cache.json")

	if ensureRef(props, "autoload", "autoload.json") {
		updated = true
	}
	if ensureRef(props, "monitoring", "monitoring.json") {
		updated = true
	}
	if ensureArrayRef(props, "tools", "tool.json") {
		updated = true
	}
	if ensureArrayRef(props, "agents", "agent.json") {
		updated = true
	}
	if ensureArrayRef(props, "mcps", "mcp.json") {
		updated = true
	}
	if ensureArrayRef(props, "memories", "memory.json") {
		updated = true
	}
	if ensureArrayRef(props, "embedders", "embedder.json") {
		updated = true
	}
	if ensureArrayRef(props, "vector_dbs", "vectordb.json") {
		updated = true
	}
	if ensureArrayRef(props, "knowledge_bases", "knowledge-base.json") {
		updated = true
	}
	if ensureArrayRef(props, "knowledge", "knowledge-binding.json") {
		updated = true
	}
	return updated
}

func updateWorkflowTriggers(schema map[string]any, props map[string]any) bool {
	defs, ok := mapValue(schema, "$defs")
	if !ok {
		return false
	}
	trigger, ok := mapValue(defs, "Trigger")
	if !ok {
		return false
	}
	trigger["oneOf"] = []any{
		map[string]any{
			"properties": map[string]any{
				"type":   map[string]any{"const": "signal"},
				"name":   mapValueWithType("string"),
				"schema": map[string]any{"$ref": "#/$defs/Schema"},
			},
			"additionalProperties": false,
			"required":             []any{"type", "name"},
		},
		map[string]any{
			"properties": map[string]any{
				"type":    map[string]any{"const": "webhook"},
				"webhook": map[string]any{"$ref": "webhook.json"},
			},
			"additionalProperties": false,
			"required":             []any{"type", "webhook"},
		},
	}
	trigger["type"] = schemaTypeObject
	trigger["discriminator"] = map[string]any{"propertyName": "type"}
	delete(trigger, "properties")
	updated := true
	if triggers, ok := mapValue(props, "triggers"); ok {
		if items, ok := mapValue(triggers, "items"); ok {
			if triggerProps, ok := mapValue(items, "properties"); ok {
				if webhookProp, ok := mapValue(triggerProps, "webhook"); ok {
					webhookProp["$ref"] = "webhook.json"
				}
			}
		}
	}
	return updated
}

func mapValue(parent map[string]any, key string) (map[string]any, bool) {
	value, ok := parent[key]
	if !ok {
		return nil, false
	}
	result, ok := value.(map[string]any)
	return result, ok
}

func ensureArrayRef(parent map[string]any, key, ref string) bool {
	prop, ok := mapValue(parent, key)
	if !ok {
		return false
	}
	items, ok := mapValue(prop, "items")
	if !ok {
		return false
	}
	if items["$ref"] == ref {
		return false
	}
	items["$ref"] = ref
	return true
}

func ensureRef(parent map[string]any, key, ref string) bool {
	prop, ok := mapValue(parent, key)
	if !ok {
		return false
	}
	if prop["$ref"] == ref {
		return false
	}
	prop["$ref"] = ref
	return true
}

func simplifyNestedTask(parent map[string]any, key, description string) bool {
	prop, ok := mapValue(parent, key)
	if !ok {
		return false
	}
	if prop["$ref"] == nil && prop["type"] == schemaTypeObject {
		return false
	}
	delete(prop, "$ref")
	prop["type"] = schemaTypeObject
	prop["additionalProperties"] = false
	if _, ok := prop["description"]; !ok {
		prop["description"] = description
	}
	return true
}

func simplifyNestedTaskArray(parent map[string]any, key, description string) bool {
	prop, ok := mapValue(parent, key)
	if !ok {
		return false
	}
	items, ok := mapValue(prop, "items")
	if !ok {
		return false
	}
	if items["$ref"] == nil && items["type"] == schemaTypeObject {
		return false
	}
	delete(items, "$ref")
	items["type"] = schemaTypeObject
	items["additionalProperties"] = false
	if _, ok := items["description"]; !ok {
		items["description"] = description
	}
	return true
}

func mapValueWithType(typeName string) map[string]any {
	return map[string]any{"type": typeName}
}

func ensureAttachmentsSchema(schema map[string]any, props map[string]any) bool {
	attProp, ok := mapValue(props, "attachments")
	if !ok {
		return false
	}
	data, err := buildAttachmentsSchema()
	if err != nil {
		return false
	}
	for k := range attProp {
		if k == "description" {
			continue
		}
		delete(attProp, k)
	}
	maps.Copy(attProp, data.arrayDefinition)
	defs := ensureDefsMap(schema)
	delete(defs, "Attachments")
	maps.Copy(defs, data.variantDefinitions)
	return true
}

func buildAttachmentsSchema() (attachmentsSchemaBundle, error) {
	attachmentsSchemaOnce.Do(func() {
		bundle := attachmentsSchemaBundle{
			arrayDefinition: map[string]any{
				"type":        "array",
				"description": "Attachments available at this configuration scope.",
			},
			variantDefinitions: make(map[string]any),
		}
		types := []struct {
			name     string
			typeName string
			instance any
		}{
			{"AttachmentImage", string(attachment.TypeImage), &attachment.ImageAttachment{}},
			{"AttachmentPDF", string(attachment.TypePDF), &attachment.PDFAttachment{}},
			{"AttachmentAudio", string(attachment.TypeAudio), &attachment.AudioAttachment{}},
			{"AttachmentVideo", string(attachment.TypeVideo), &attachment.VideoAttachment{}},
			{"AttachmentURL", string(attachment.TypeURL), &attachment.URLAttachment{}},
			{"AttachmentFile", string(attachment.TypeFile), &attachment.FileAttachment{}},
		}
		reflector := newJSONSchemaReflector()
		oneOf := make([]any, 0, len(types))
		for _, t := range types {
			schema := reflector.Reflect(t.instance)
			variantMap, err := marshalSchema(schema)
			if err != nil {
				attachmentsSchemaErr = err
				return
			}
			ensureObjectSchema(variantMap)
			props, _ := mapValue(variantMap, "properties")
			if props == nil {
				props = make(map[string]any)
				variantMap["properties"] = props
			}
			typeProp := map[string]any{"const": t.typeName, "type": "string"}
			props["type"] = typeProp
			requiredAny, ok := variantMap["required"].([]any)
			if !ok {
				requiredAny = nil
			}
			requiredAny = appendIfMissing(requiredAny, "type")
			variantMap["required"] = requiredAny
			variantMap["description"] = buildAttachmentDescription(t.typeName)
			bundle.variantDefinitions[t.name] = variantMap
			oneOf = append(oneOf, map[string]any{"$ref": "#/$defs/" + t.name})
		}
		bundle.arrayDefinition["items"] = map[string]any{
			"oneOf":         oneOf,
			"discriminator": map[string]any{"propertyName": "type"},
		}
		attachmentsSchemaData = bundle
	})
	return attachmentsSchemaData, attachmentsSchemaErr
}

func ensureDefsMap(schema map[string]any) map[string]any {
	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		defs = make(map[string]any)
		schema["$defs"] = defs
	}
	return defs
}

func marshalSchema(schema *jsonschema.Schema) (map[string]any, error) {
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func ensureObjectSchema(schema map[string]any) {
	if schema == nil {
		return
	}
	schema["type"] = schemaTypeObject
	if _, ok := schema["additionalProperties"]; !ok {
		schema["additionalProperties"] = false
	}
}

func appendIfMissing(list []any, value string) []any {
	for _, v := range list {
		if s, ok := v.(string); ok && s == value {
			return list
		}
	}
	return append(list, value)
}

func buildAttachmentDescription(typeName string) string {
	switch attachment.Type(typeName) {
	case attachment.TypeImage:
		return "Image attachment supporting URL or path sources."
	case attachment.TypePDF:
		return "PDF attachment with optional page limits and multiple sources."
	case attachment.TypeAudio:
		return "Audio attachment supporting URL or path sources."
	case attachment.TypeVideo:
		return "Video attachment supporting URL or path sources."
	case attachment.TypeURL:
		return "External URL attachment."
	case attachment.TypeFile:
		return "Local file attachment resolved from the project workspace."
	default:
		return "Attachment configuration."
	}
}
