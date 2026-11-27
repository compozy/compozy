package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// ArchiveStoppedListLimit caps the skipped and failed threads a report lists;
// its counts stay complete.
const ArchiveStoppedListLimit = 100

// ArchiveStoppedFailureCode is the code of a thread whose archive failed unexpectedly.
const ArchiveStoppedFailureCode = "archive_failed"

// ErrArchiveStoppedInvalidRequest reports an Archive stopped request without exactly one scope.
var ErrArchiveStoppedInvalidRequest = errors.New("session: archive stopped requires one profile and one scope")

// ArchiveStoppedRequest archives every eligible stopped thread of one profile,
// in one workspace or in all of its unarchived workspaces.
type ArchiveStoppedRequest struct {
	ProfileID     string
	WorkspaceID   string // exactly one of WorkspaceID / AllWorkspaces
	AllWorkspaces bool
	DryRun        bool
	// CallerSessionID is the native tool caller; its thread is skipped as "caller".
	CallerSessionID string
}

// ArchiveStoppedSkipReason is the stable code of why a thread was skipped.
type ArchiveStoppedSkipReason string

// The store's ArchiveStoppedBlock values share these codes.
const (
	ArchiveStoppedSkipCaller       ArchiveStoppedSkipReason = "caller"
	ArchiveStoppedSkipInstanceMain ArchiveStoppedSkipReason = "instance_main"
	ArchiveStoppedSkipRunning      ArchiveStoppedSkipReason = "running"
	ArchiveStoppedSkipWaitingOnYou ArchiveStoppedSkipReason = "waiting_on_you"
	ArchiveStoppedSkipQueuedInput  ArchiveStoppedSkipReason = "queued_input"
	ArchiveStoppedSkipPinned       ArchiveStoppedSkipReason = "pinned"
	ArchiveStoppedSkipLater        ArchiveStoppedSkipReason = "later"
)

// ArchiveStoppedThread is one archived thread, or one a dry run would archive.
type ArchiveStoppedThread struct {
	RootID      string
	WorkspaceID string
	Title       string
	// SessionIDs are the members this operation archived, root first.
	SessionIDs []string
	// RestoreIDs are SessionIDs without subagents, root first: unarchiving
	// them in order undoes the archive, as each subagent restores with its parent.
	RestoreIDs []string
}

// ArchiveStoppedSkip is one thread the operation left alone.
type ArchiveStoppedSkip struct {
	RootID      string
	WorkspaceID string
	Title       string
	Reason      ArchiveStoppedSkipReason
}

// ArchiveStoppedFailure is one thread whose archive failed unexpectedly.
type ArchiveStoppedFailure struct {
	RootID      string
	WorkspaceID string
	Title       string
	Code        string
	Message     string
}

// ArchiveStoppedReport is the outcome of one Archive stopped operation.
type ArchiveStoppedReport struct {
	DryRun            bool
	Archived          []ArchiveStoppedThread
	SkipCounts        map[ArchiveStoppedSkipReason]int
	Skipped           []ArchiveStoppedSkip // first ArchiveStoppedListLimit, in thread order
	SkippedTruncated  bool
	Failures          []ArchiveStoppedFailure // first ArchiveStoppedListLimit, in thread order
	FailuresTruncated bool
	// FailureCount counts every failed thread, listed or not.
	FailureCount int
}

// ArchivedSessionCount is how many sessions the report archived.
func (r *ArchiveStoppedReport) ArchivedSessionCount() int {
	total := 0
	for i := range r.Archived {
		total += len(r.Archived[i].SessionIDs)
	}
	return total
}

// ArchiveStopped archives, whole, every thread of the scope whose members are
// all stopped and none needs the operator; it never stops a session. Each
// thread archives in its own transaction that re-checks every rule, so one
// thread's failure leaves the others archived. A dry run archives nothing.
// A request that ends early, such as a client disconnect, does not stop it
// partway: the archived ids the report returns are the only Undo there is.
func (m *Manager) ArchiveStopped(ctx context.Context, req ArchiveStoppedRequest) (ArchiveStoppedReport, error) {
	ctx = context.WithoutCancel(ctx)
	req.ProfileID = strings.TrimSpace(req.ProfileID)
	req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	req.CallerSessionID = strings.TrimSpace(req.CallerSessionID)
	if req.ProfileID == "" || (req.WorkspaceID != "") == req.AllWorkspaces {
		return ArchiveStoppedReport{}, ErrArchiveStoppedInvalidRequest
	}
	stoppedStore, ok := m.sessionCatalog.(store.SessionArchiveStoppedStore)
	if !ok || stoppedStore == nil {
		return ArchiveStoppedReport{}, ErrSessionArchiveUnavailable
	}
	members, err := stoppedStore.ListArchiveStoppedMembers(ctx, store.ArchiveStoppedQuery{
		ProfileID: req.ProfileID, WorkspaceID: req.WorkspaceID,
	})
	if err != nil {
		return ArchiveStoppedReport{}, err
	}
	report := ArchiveStoppedReport{DryRun: req.DryRun, SkipCounts: map[ArchiveStoppedSkipReason]int{}}
	for _, thread := range archiveStoppedThreads(members) {
		m.archiveStoppedThread(ctx, req, thread, &report)
	}
	m.logger.Info("session: archive stopped",
		"profile_id", req.ProfileID,
		"workspace_id", req.WorkspaceID,
		"all_workspaces", req.AllWorkspaces,
		"dry_run", req.DryRun,
		"archived_threads", len(report.Archived),
		"archived_sessions", report.ArchivedSessionCount(),
		"skip_counts", report.SkipCounts,
		"failures", report.FailureCount,
	)
	return report, nil
}

// archiveStoppedThreads groups members by thread; the store lists them grouped, root first.
func archiveStoppedThreads(members []store.ArchiveStoppedMember) [][]store.ArchiveStoppedMember {
	threads := make([][]store.ArchiveStoppedMember, 0)
	for start := 0; start < len(members); {
		end := start + 1
		for end < len(members) && members[end].RootID == members[start].RootID {
			end++
		}
		threads = append(threads, members[start:end])
		start = end
	}
	return threads
}

func (m *Manager) archiveStoppedThread(
	ctx context.Context,
	req ArchiveStoppedRequest,
	members []store.ArchiveStoppedMember,
	report *ArchiveStoppedReport,
) {
	root := &members[0]
	ids := make([]string, 0, len(members))
	subagents := map[string]bool{}
	for i := range members {
		ids = append(ids, members[i].SessionID)
		subagents[members[i].SessionID] = members[i].Subagent
	}
	if reason := m.archiveStoppedSkipReason(req.CallerSessionID, members, ids); reason != "" {
		report.skip(root, reason)
		return
	}
	if req.DryRun {
		report.archive(root, ids, subagents)
		return
	}
	tree, err := m.ArchiveWithChildren(ctx, root.WorkspaceID, root.SessionID, ArchiveTreeOptions{
		RefuseMember: req.CallerSessionID,
		StoppedOnly:  true,
	})
	if err == nil {
		for _, member := range tree.Descendants {
			if member.Lineage != nil && member.Lineage.SpawnRole == store.SubagentSpawnRole {
				subagents[member.ID] = true
			}
		}
		report.archive(root, archivedTreeIDs(ids, tree), subagents)
		return
	}
	if reason := archiveStoppedRefusalReason(err); reason != "" {
		report.skip(root, reason)
		return
	}
	// A thread archived, deleted or re-parented since the read is no longer a candidate.
	if errors.Is(err, store.ErrArchiveStoppedNotCandidate) || errors.Is(err, ErrSessionNotFound) ||
		errors.Is(err, store.ErrSessionNotFound) {
		return
	}
	report.fail(root, err)
}

// archivedTreeIDs orders what the archive took as the dry run lists it: the
// read members first, root first, then members that joined the thread since.
func archivedTreeIDs(read []string, tree *Tree) []string {
	taken := map[string]bool{tree.Root.ID: true}
	for _, member := range tree.Descendants {
		taken[member.ID] = true
	}
	ids := make([]string, 0, len(taken))
	for _, id := range read {
		if taken[id] {
			ids = append(ids, id)
			delete(taken, id)
		}
	}
	for _, member := range tree.Descendants {
		if taken[member.ID] {
			ids = append(ids, member.ID)
		}
	}
	return ids
}

// archiveStoppedSkipReason is the first rule the thread breaks before any
// archive attempt: the caller's own thread, then the store rules, then a
// resume in flight, which counts as running.
func (m *Manager) archiveStoppedSkipReason(
	callerID string,
	members []store.ArchiveStoppedMember,
	ids []string,
) ArchiveStoppedSkipReason {
	for _, id := range ids {
		if callerID != "" && id == callerID {
			return ArchiveStoppedSkipCaller
		}
	}
	block := store.ArchiveStoppedThreadBlock(members)
	if block == store.ArchiveStoppedBlockNone && m.resumeInFlight(ids) {
		block = store.ArchiveStoppedBlockRunning
	}
	return ArchiveStoppedSkipReason(block)
}

func (m *Manager) resumeInFlight(ids []string) bool {
	m.resumeLifecycle.mu.Lock()
	defer m.resumeLifecycle.mu.Unlock()
	for _, id := range ids {
		if m.resumeLifecycle.runs[id] != nil {
			return true
		}
	}
	return false
}

// archiveStoppedRefusalReason maps a refusal from the archive transaction to
// the thread's current skip reason.
func archiveStoppedRefusalReason(err error) ArchiveStoppedSkipReason {
	if ineligible, ok := errors.AsType[*store.ArchiveStoppedIneligibleError](err); ok {
		return ArchiveStoppedSkipReason(ineligible.Block)
	}
	switch {
	case errors.Is(err, ErrSessionTreeContainsCaller):
		return ArchiveStoppedSkipCaller
	case errors.Is(err, store.ErrSessionIsInstanceMain):
		return ArchiveStoppedSkipInstanceMain
	case errors.Is(err, ErrSessionArchiveRequiresStopped), errors.Is(err, store.ErrSessionArchiveRequiresStopped):
		return ArchiveStoppedSkipRunning
	default:
		return ""
	}
}

func (r *ArchiveStoppedReport) archive(root *store.ArchiveStoppedMember, ids []string, subagents map[string]bool) {
	restore := make([]string, 0, len(ids))
	for _, id := range ids {
		if !subagents[id] {
			restore = append(restore, id)
		}
	}
	r.Archived = append(r.Archived, ArchiveStoppedThread{
		RootID: root.SessionID, WorkspaceID: root.WorkspaceID, Title: root.Name, SessionIDs: ids, RestoreIDs: restore,
	})
}

func (r *ArchiveStoppedReport) skip(root *store.ArchiveStoppedMember, reason ArchiveStoppedSkipReason) {
	r.SkipCounts[reason]++
	if len(r.Skipped) == ArchiveStoppedListLimit {
		r.SkippedTruncated = true
		return
	}
	r.Skipped = append(r.Skipped, ArchiveStoppedSkip{
		RootID: root.SessionID, WorkspaceID: root.WorkspaceID, Title: root.Name, Reason: reason,
	})
}

func (r *ArchiveStoppedReport) fail(root *store.ArchiveStoppedMember, err error) {
	r.FailureCount++
	if len(r.Failures) == ArchiveStoppedListLimit {
		r.FailuresTruncated = true
		return
	}
	r.Failures = append(r.Failures, ArchiveStoppedFailure{
		RootID:      root.SessionID,
		WorkspaceID: root.WorkspaceID,
		Title:       root.Name,
		Code:        ArchiveStoppedFailureCode,
		Message:     fmt.Sprintf("archive thread %s: %v", root.SessionID, err),
	})
}
