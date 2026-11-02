package store

// SessionWorkLink retains identity independently of worktree lifetime. Unlinked
// rows are tombstones so automatic discovery cannot undo a user's removal.
type SessionWorkLink struct {
	SessionID       string
	WorkspaceID     string
	Repository      string
	Branch          string
	PRNumber        int
	WorktreeID      string
	WorktreeName    string
	Provenance      string
	Unlinked        bool
	WorktreeRemoved bool
}

type SessionWorkMember struct{ RootID, ID, WorkspaceID, ProfileID, Name, WorktreeID string }
