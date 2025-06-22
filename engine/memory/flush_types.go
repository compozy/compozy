package memory

// FlushMemoryActivityInput contains the necessary information to perform a memory flush.
type FlushMemoryActivityInput struct {
	MemoryInstanceKey    string // The unique key for the memory instance (resolved key)
	MemoryResourceID     string // The ID of the MemoryResource config
	ProjectID            string // The project ID for namespacing
	ForceFlush           bool   // Whether to force flush regardless of conditions
	ReportProgressTaskID string // Optional: Task ID for progress reporting
}

// FlushMemoryActivityOutput contains the result of a memory flush operation.
type FlushMemoryActivityOutput struct {
	Success          bool   `json:"success"`
	SummaryGenerated bool   `json:"summary_generated"`
	MessageCount     int    `json:"message_count"`
	TokenCount       int    `json:"token_count"`
	Error            string `json:"error,omitempty"`
}
