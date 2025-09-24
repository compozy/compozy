package httpdto

// PageInfoDTO defines the standard pagination block embedded in list responses.
//
// Naming conventions for DTOs across resources:
// - <Resource>DTO: Single item representation (e.g., ToolDTO)
// - <Resource>ListItem: Item shape in list collections
// - <Resource>ListResponse: Envelope with collection and PageInfoDTO
//
// Mappers that build DTOs MUST remain pure and never import HTTP/router frameworks
// like gin. Keep request handling concerns in handlers and use this DTO from
// resource routers. Response envelopes continue to use router.Response.
type PageInfoDTO struct {
	Limit      int    `json:"limit"                 example:"50"`
	Total      int    `json:"total,omitempty"       example:"2"`
	NextCursor string `json:"next_cursor,omitempty" example:"v2:after:tool-001"`
	PrevCursor string `json:"prev_cursor,omitempty" example:"v2:before:tool-000"`
}

// AsString converts an interface{} to a string safely.
func AsString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// AsMap converts an interface{} to map[string]any safely.
func AsMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}
