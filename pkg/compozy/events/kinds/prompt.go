package kinds

// PromptBuiltPayload describes a prompt built for a job.
type PromptBuiltPayload struct {
	Index     int      `json:"index"`
	SafeName  string   `json:"safe_name,omitempty"`
	TaskTitle string   `json:"task_title,omitempty"`
	TaskType  string   `json:"task_type,omitempty"`
	CodeFiles []string `json:"code_files,omitempty"`
}

// PromptWrittenPayload describes a prompt persisted to disk.
type PromptWrittenPayload struct {
	Index    int    `json:"index"`
	SafeName string `json:"safe_name,omitempty"`
	Path     string `json:"path,omitempty"`
	Bytes    int    `json:"bytes,omitempty"`
}
