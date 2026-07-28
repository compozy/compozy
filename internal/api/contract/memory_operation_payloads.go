package contract

import "time"

// MemoryWriteRequest is the shared memory write request payload.
type MemoryWriteRequest struct {
	Content   string `json:"content"`
	Scope     string `json:"scope,omitempty"`
	Workspace string `json:"workspace,omitempty"`
}

// MemoryReadResponse is the shared memory read response payload.
type MemoryReadResponse struct {
	Content string `json:"content"`
}

// MemoryMutationResponse is the shared memory mutation response payload.
type MemoryMutationResponse struct {
	OK bool `json:"ok"`
}

// MemoryConsolidateRequest is the shared memory consolidation request payload.
type MemoryConsolidateRequest struct {
	Workspace string `json:"workspace,omitempty"`
}

// MemoryConsolidateResponse is the shared memory consolidation response payload.
type MemoryConsolidateResponse struct {
	Triggered bool   `json:"triggered"`
	Reason    string `json:"reason,omitempty"`
}

// MemoryReindexRequest is the shared memory-catalog reindex request payload.
type MemoryReindexRequest struct {
	Scope     string `json:"scope,omitempty"`
	Workspace string `json:"workspace,omitempty"`
}

// MemoryOperationPayload is one redacted memory operation history row.
type MemoryOperationPayload struct {
	ID        string    `json:"id"`
	Operation string    `json:"operation"`
	Scope     string    `json:"scope,omitempty"`
	Workspace string    `json:"workspace,omitempty"`
	Filename  string    `json:"filename,omitempty"`
	AgentName string    `json:"agent_name,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// MemoryHistoryResponse wraps the bounded memory operation history payload.
type MemoryHistoryResponse struct {
	Operations []MemoryOperationPayload `json:"operations"`
}

// MemoryHealthPayload is the shared memory health response payload.
type MemoryHealthPayload struct {
	Status             string     `json:"status"`
	Reason             string     `json:"reason,omitempty"`
	Enabled            bool       `json:"enabled"`
	Configured         bool       `json:"configured"`
	GlobalDir          string     `json:"global_dir,omitempty"`
	GlobalFiles        int        `json:"global_files"`
	WorkspaceFiles     int        `json:"workspace_files"`
	WorkspaceCount     int        `json:"workspace_count"`
	DreamEnabled       bool       `json:"dream_enabled"`
	DreamAgent         string     `json:"dream_agent,omitempty"`
	DreamMinHours      float64    `json:"dream_min_hours,omitempty"`
	DreamMinSessions   int        `json:"dream_min_sessions,omitempty"`
	DreamCheckInterval string     `json:"dream_check_interval,omitempty"`
	IndexedFiles       int        `json:"indexed_files"`
	OrphanedFiles      int        `json:"orphaned_files"`
	LastReindex        *time.Time `json:"last_reindex"`
	OperationCount     int        `json:"operation_count"`
	LastOperationAt    *time.Time `json:"last_operation_at"`
	LastConsolidation  *time.Time `json:"last_consolidation"`
}
