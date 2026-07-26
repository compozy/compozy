package contract

// MemoryListResponse wraps one counted page of Memory v2 headers.
type MemoryListResponse struct {
	Memories []MemoryEntrySummaryPayload `json:"memories"`
	Page     CountedCursorPagePayload    `json:"page"`
}
