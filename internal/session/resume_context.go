package session

import "context"

// ResumeContextProvider supplies workspace continuity that must accompany a
// degraded resume replay without coupling session lifecycle code to memory.
type ResumeContextProvider interface {
	ResumeContextSection(ctx context.Context, startup StartupPromptContext) (string, error)
}
