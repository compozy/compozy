package memrouter

// MemoryKeyRequest is a common request structure for operations that require a key
type MemoryKeyRequest struct {
	Key string `json:"key" binding:"required"`
}

// WriteMemoryRequest extends the write input with key
type WriteMemoryRequest struct {
	Key      string           `json:"key"      binding:"required"`
	Messages []map[string]any `json:"messages" binding:"required"`
}

// AppendMemoryRequest extends the append input with key
type AppendMemoryRequest struct {
	Key      string           `json:"key"      binding:"required"`
	Messages []map[string]any `json:"messages" binding:"required"`
}

// DeleteMemoryRequest contains the key for deletion
type DeleteMemoryRequest struct {
	Key string `json:"key" binding:"required"`
}

// FlushMemoryRequest extends the flush input with key
type FlushMemoryRequest struct {
	Key      string `json:"key"                binding:"required"`
	Force    bool   `json:"force,omitempty"`
	DryRun   bool   `json:"dry_run,omitempty"`
	MaxKeys  int    `json:"max_keys,omitempty"`
	Strategy string `json:"strategy,omitempty"`
}

// ClearMemoryRequest extends the clear input with key
type ClearMemoryRequest struct {
	Key     string `json:"key"              binding:"required"`
	Confirm bool   `json:"confirm"          binding:"required"`
	Backup  bool   `json:"backup,omitempty"`
}
