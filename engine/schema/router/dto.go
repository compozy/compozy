package schemarouter

import (
	"encoding/json"

	"github.com/compozy/compozy/engine/infra/server/router"
)

// SchemaDTO wraps the JSON Schema body as raw JSON bytes to keep a stable
// transport shape without deep decoding on the server side.
type SchemaDTO struct {
	Body json.RawMessage `json:"body"`
}

// SchemaListItem is the list representation with optional strong ETag.
type SchemaListItem struct {
	Body json.RawMessage `json:"body"`
	ETag string          `json:"etag,omitempty" example:"abc123"`
}

// SchemasListResponse is the typed list payload returned from GET /schemas.
type SchemasListResponse struct {
	Schemas []SchemaListItem   `json:"schemas"`
	Page    router.PageInfoDTO `json:"page"`
}

// toSchemaDTO marshals a dynamic map to SchemaDTO.
func toSchemaDTO(src map[string]any) (SchemaDTO, int, error) {
	b, err := json.Marshal(src)
	if err != nil {
		return SchemaDTO{}, 0, err
	}
	return SchemaDTO{Body: json.RawMessage(b)}, len(b), nil
}

// toSchemaListItem marshals a dynamic map to SchemaListItem, normalizing _etag → etag.
func toSchemaListItem(src map[string]any) (SchemaListItem, int, error) {
	b, err := json.Marshal(src)
	if err != nil {
		return SchemaListItem{}, 0, err
	}
	return SchemaListItem{Body: json.RawMessage(b), ETag: router.AsString(src["_etag"])}, len(b), nil
}
