package contract

// CursorPagePayload describes one bounded stable-cursor page. NextCursor
// continues in the direction of the request. Limit is the normalized bound
// applied by the server.
type CursorPagePayload struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Limit      int    `json:"limit"`
}

// CountedCursorPagePayload describes a bounded catalog page. Total is the
// number of rows matching the workspace-scoped filters before the cursor is
// applied, including zero for an empty catalog.
type CountedCursorPagePayload struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Total      int    `json:"total"`
	Limit      int    `json:"limit"`
}
