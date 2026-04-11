package kinds

import "encoding/json"

// ArtifactUpdatedPayload describes a host-managed artifact write.
type ArtifactUpdatedPayload struct {
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written,omitempty"`
}

// ExtensionLoadedPayload describes an extension entering the manager lifecycle.
type ExtensionLoadedPayload struct {
	Extension    string `json:"extension"`
	Source       string `json:"source,omitempty"`
	Version      string `json:"version,omitempty"`
	ManifestPath string `json:"manifest_path,omitempty"`
}

// ExtensionReadyPayload describes a successfully initialized extension session.
type ExtensionReadyPayload struct {
	Extension            string   `json:"extension"`
	Source               string   `json:"source,omitempty"`
	Version              string   `json:"version,omitempty"`
	ProtocolVersion      string   `json:"protocol_version"`
	AcceptedCapabilities []string `json:"accepted_capabilities,omitempty"`
	SupportedHookEvents  []string `json:"supported_hook_events,omitempty"`
}

// ExtensionFailedPayload describes a lifecycle failure for one extension.
type ExtensionFailedPayload struct {
	Extension string `json:"extension"`
	Source    string `json:"source,omitempty"`
	Version   string `json:"version,omitempty"`
	Phase     string `json:"phase"`
	Error     string `json:"error"`
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
