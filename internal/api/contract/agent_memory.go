package contract

// Agent memory surfaces are experimental for one release.
type AgentMemoryBudget struct {
	Lines    int `json:"lines"`
	MaxLines int `json:"max_lines"`
	Bytes    int `json:"bytes"`
	MaxBytes int `json:"max_bytes"`
}

type AgentMemoryDocument struct {
	Content string            `json:"content"`
	Digest  string            `json:"digest"`
	Budget  AgentMemoryBudget `json:"budget"`
}

type AgentMemoryWriteRequest struct {
	Content        string `json:"content"`
	ExpectedDigest string `json:"expected_digest"`
}

type AgentMemoryAddRequest struct {
	Text  string `json:"text"`
	Until string `json:"until,omitempty"`
}

type AgentMemoryReplaceRequest struct {
	Match string `json:"match"`
	Text  string `json:"text"`
}

type AgentMemoryRemoveRequest struct {
	Match string `json:"match"`
}
type AgentMemoryRemoveResponse struct {
	Digest  string `json:"digest"`
	Removed int    `json:"removed"`
}
type AgentMemorySearchRequest struct {
	Query           string `json:"query"`
	IncludeSessions bool   `json:"include_sessions,omitempty"`
}
type AgentMemorySearchResult struct {
	Source    string  `json:"source"`
	Text      string  `json:"text"`
	File      string  `json:"file,omitempty"`
	Line      int     `json:"line,omitempty"`
	Score     float64 `json:"score,omitempty"`
	SessionID string  `json:"session_id,omitempty"`
}
type AgentMemorySearchResponse struct {
	Results []AgentMemorySearchResult `json:"results"`
}
type AgentMemoryValidateRequest struct {
	Content string `json:"content"`
}
type AgentMemoryValidateResponse struct {
	Valid       bool     `json:"valid"`
	Diagnostics []string `json:"diagnostics"`
}
type AgentMemoryRevision struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}
type AgentMemoryHistoryResponse struct {
	Revisions []AgentMemoryRevision `json:"revisions"`
}
type AgentMemoryRollbackRequest struct {
	Revision string `json:"revision"`
}
