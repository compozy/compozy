package contract

// ArchiveSessionRequest optionally stops a live session before archiving it.
type ArchiveSessionRequest struct {
	// Stop archives a live session in one action by stopping it first through
	// the normal stop path. Without it, archiving a live session is refused.
	Stop bool `json:"stop,omitempty"`
}
