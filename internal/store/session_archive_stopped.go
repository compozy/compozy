package store

import (
	"context"
	"errors"
)

// ErrArchiveStoppedNotCandidate reports that a thread root is no longer an
// unarchived thread root in scope, so Archive stopped has nothing to archive.
var ErrArchiveStoppedNotCandidate = errors.New("store: session is not an archive stopped candidate")

// ArchiveStoppedBlock is why Archive stopped leaves a thread alone. The values
// are the public skip reason codes, in the order a thread is classified.
type ArchiveStoppedBlock string

const (
	ArchiveStoppedBlockNone         ArchiveStoppedBlock = ""
	ArchiveStoppedBlockInstanceMain ArchiveStoppedBlock = "instance_main"
	ArchiveStoppedBlockRunning      ArchiveStoppedBlock = "running"
	ArchiveStoppedBlockWaitingOnYou ArchiveStoppedBlock = "waiting_on_you"
	ArchiveStoppedBlockQueuedInput  ArchiveStoppedBlock = "queued_input"
	ArchiveStoppedBlockPinned       ArchiveStoppedBlock = "pinned"
	ArchiveStoppedBlockLater        ArchiveStoppedBlock = "later"
)

// ArchiveStoppedIneligibleError refuses the archive of a thread that a rule
// blocks at the moment its archive transaction runs.
type ArchiveStoppedIneligibleError struct {
	RootID string
	Block  ArchiveStoppedBlock
}

func (e *ArchiveStoppedIneligibleError) Error() string {
	return "store: archive stopped thread " + e.RootID + " is blocked: " + string(e.Block)
}

// ArchiveStoppedQuery selects the threads of one profile in one workspace, or
// in every unarchived workspace when WorkspaceID is empty.
type ArchiveStoppedQuery struct {
	ProfileID   string
	WorkspaceID string
}

// ArchiveStoppedMember is one unarchived member of a candidate thread with the
// facts that block archiving it. Archived members are never listed.
type ArchiveStoppedMember struct {
	RootID       string
	SessionID    string
	WorkspaceID  string
	Name         string
	State        string
	InstanceMain bool
	WaitingOnYou bool
	QueuedInput  bool
	Pinned       bool
	Later        bool
	// Subagent members restore with their parent's unarchive, never on their own.
	Subagent bool
}

// SessionArchiveStoppedStore lists Archive stopped candidates and archives one
// thread after re-checking every rule inside the archive transaction.
type SessionArchiveStoppedStore interface {
	// ListArchiveStoppedMembers returns members grouped by thread, root first.
	ListArchiveStoppedMembers(ctx context.Context, query ArchiveStoppedQuery) ([]ArchiveStoppedMember, error)
	// ArchiveStoppedSessionTree archives one thread only while ArchiveStoppedThreadBlock
	// finds nothing; the result lists only the members this call archived.
	ArchiveStoppedSessionTree(ctx context.Context, workspaceID string, rootID string) (SessionTreeArchive, error)
}

// ArchiveStoppedThreadBlock classifies one thread's members: the first rule,
// in skip-reason order, that any member breaks.
func ArchiveStoppedThreadBlock(members []ArchiveStoppedMember) ArchiveStoppedBlock {
	checks := []struct {
		block   ArchiveStoppedBlock
		blocked func(*ArchiveStoppedMember) bool
	}{
		{ArchiveStoppedBlockInstanceMain, func(m *ArchiveStoppedMember) bool { return m.InstanceMain }},
		{ArchiveStoppedBlockRunning, func(m *ArchiveStoppedMember) bool { return m.State != "stopped" }},
		{ArchiveStoppedBlockWaitingOnYou, func(m *ArchiveStoppedMember) bool { return m.WaitingOnYou }},
		{ArchiveStoppedBlockQueuedInput, func(m *ArchiveStoppedMember) bool { return m.QueuedInput }},
		{ArchiveStoppedBlockPinned, func(m *ArchiveStoppedMember) bool { return m.Pinned }},
		{ArchiveStoppedBlockLater, func(m *ArchiveStoppedMember) bool { return m.Later }},
	}
	for _, check := range checks {
		for i := range members {
			if check.blocked(&members[i]) {
				return check.block
			}
		}
	}
	return ArchiveStoppedBlockNone
}
