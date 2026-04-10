package kinds

import "encoding/json"

// ArtifactUpdatedPayload describes a host-managed artifact write.
type ArtifactUpdatedPayload struct {
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written,omitempty"`
}

// TaskMemoryUpdatedPayload describes a workflow or task memory document write.
type TaskMemoryUpdatedPayload struct {
	Workflow     string `json:"workflow,omitempty"`
	TaskFile     string `json:"task_file,omitempty"`
	Path         string `json:"path"`
	Mode         string `json:"mode,omitempty"`
	BytesWritten int    `json:"bytes_written,omitempty"`
}

// ExtensionEventPayload describes a custom event emitted through host.events.publish.
type ExtensionEventPayload struct {
	Extension string          `json:"extension,omitempty"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}
