// Package mention parses @-mention tokens from prompt text. It does no I/O.
//
// A Token is a syntactic candidate; only the session layer turns tokens into
// verified Mentions (working-directory containment for paths, same-workspace
// lookup for sessions).
package mention

// Kind names what a verified mention points at.
type Kind string

const (
	// KindFile is a regular file under the target session's working directory.
	KindFile Kind = "file"
	// KindFolder is a directory under the target session's working directory.
	KindFolder Kind = "folder"
	// KindSession is another session of the target session's workspace and profile.
	KindSession Kind = "session"
)

const (
	// MaxCandidateTokens bounds how many candidate tokens one prompt examines.
	MaxCandidateTokens = 200
	// MaxPathMentions bounds distinct file/folder targets resolved per prompt.
	MaxPathMentions = 50
	// MaxSessionMentions bounds distinct session targets resolved per prompt.
	MaxSessionMentions = 16
)

// Token is a syntactic candidate; it is not a mention until resolved.
type Token struct {
	Raw       string // exact text, e.g. `@"docs/My Notes.md"` or `@session:sess-9f8e7d`
	Path      string // unquoted path for path tokens; empty for session tokens
	SessionID string // for session tokens
	Start     int    // UTF-16 offset, inclusive
	End       int    // UTF-16 offset, exclusive
}

// Mention is a resolved, verified token.
type Mention struct {
	Kind   Kind   `json:"kind"`
	Token  string `json:"token"`
	Target string `json:"target"` // relative path (folders end in "/") or session id
	Label  string `json:"label"`  // file/folder name or session title at admission
	Start  int    `json:"start"`
	End    int    `json:"end"`
}
