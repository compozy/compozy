// Package sessioncleanup plans and runs the bulk deletion of a profile's
// archived session history: whole archived threads, or only their child
// sessions or subagents, picked by the day the thread was archived.
package sessioncleanup

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// Selection is what one cleanup deletes. Everything deletes each picked
// archived thread whole, so it includes the other two; without it, Children and
// Subagents thin the threads out and keep their main sessions.
type Selection struct {
	Everything bool
	Children   bool
	Subagents  bool
}

// Empty reports a selection that deletes nothing.
func (s Selection) Empty() bool {
	return !s.Everything && !s.Children && !s.Subagents
}

// Scope is the archived history one cleanup reads: one profile, in one
// workspace (or the user selector) or across all its workspaces.
type Scope struct {
	ProfileID     string
	WorkspaceID   string
	AllWorkspaces bool
}

// Request is one cleanup: what to delete, from which threads.
type Request struct {
	Scope     Scope
	Selection Selection
	// ArchivedBefore picks threads whose main session was archived strictly
	// before it; nil picks every archived thread.
	ArchivedBefore *time.Time
	// RootIDs, when set, narrows the picked threads to these main sessions (a retry).
	RootIDs []string
}

// Counts is how many sessions a cleanup deletes, by kind.
type Counts struct {
	// Sessions are archived main sessions (Everything only).
	Sessions  int
	Children  int
	Subagents int
}

// Total is every session the counts delete.
func (c Counts) Total() int {
	return c.Sessions + c.Children + c.Subagents
}

func (c *Counts) add(other Counts) {
	c.Sessions += other.Sessions
	c.Children += other.Children
	c.Subagents += other.Subagents
}

// WorkspaceTotal is how many sessions a cleanup deletes in one workspace.
type WorkspaceTotal struct {
	WorkspaceID string
	Total       int
}

// Thread is one picked archived thread and the sessions deleted from it,
// deepest first, so a failure part-way leaves every remaining session
// reachable from the thread's main session.
type Thread struct {
	RootID      string
	WorkspaceID string
	Title       string
	Delete      []string
	Counts      Counts
}

// Plan is the dry run of one cleanup.
type Plan struct {
	Counts Counts
	// KeptLive counts child sessions still in the session list or in Later that
	// the selection would otherwise delete; they and everything under them stay.
	KeptLive int
	// Choices are the totals each check alone would delete in the same range.
	Choices    ChoiceTotals
	Workspaces []WorkspaceTotal
	Threads    []Thread
}

// ChoiceTotals are the per-check totals a picker shows beside each check.
type ChoiceTotals struct {
	Everything int
	Children   int
	Subagents  int
}

// cleanupNode is one row placed in its thread's hierarchy.
type cleanupNode struct {
	row      store.SessionHistoryCleanupRow
	children []*cleanupNode
}

func (n *cleanupNode) archived() bool {
	return n.row.ArchivedAt != nil
}

func (n *cleanupNode) subagent() bool {
	return strings.EqualFold(strings.TrimSpace(n.row.SpawnRole), store.SubagentSpawnRole)
}

// BuildPlan places the rows in their threads and decides, for every picked
// thread, which sessions go. It is pure: the same rows always plan the same way.
func BuildPlan(rows []store.SessionHistoryCleanupRow, request Request) Plan {
	roots := pickThreads(buildForest(rows), request)
	plan := Plan{
		Choices: ChoiceTotals{
			Everything: planCounts(roots, Selection{Everything: true}).Total(),
			Children:   planCounts(roots, Selection{Children: true}).Total(),
			Subagents:  planCounts(roots, Selection{Subagents: true}).Total(),
		},
	}
	byWorkspace := make(map[string]int)
	for _, root := range roots {
		thread, kept := planThread(root, request.Selection)
		plan.KeptLive += kept
		if len(thread.Delete) == 0 {
			continue
		}
		plan.Counts.add(thread.Counts)
		byWorkspace[thread.WorkspaceID] += thread.Counts.Total()
		plan.Threads = append(plan.Threads, thread)
	}
	for workspaceID, total := range byWorkspace {
		plan.Workspaces = append(plan.Workspaces, WorkspaceTotal{WorkspaceID: workspaceID, Total: total})
	}
	slices.SortFunc(plan.Workspaces, func(a, b WorkspaceTotal) int {
		return cmp.Or(cmp.Compare(b.Total, a.Total), cmp.Compare(a.WorkspaceID, b.WorkspaceID))
	})
	return plan
}

// buildForest links each row to its list parent: the parent in the same
// workspace, unless the row continues or forks it, which makes it a thread of
// its own. It returns the archived rows that head a thread in the Archived list.
func buildForest(rows []store.SessionHistoryCleanupRow) []*cleanupNode {
	nodes := make(map[string]*cleanupNode, len(rows))
	for i := range rows {
		nodes[rows[i].ID] = &cleanupNode{row: rows[i]}
	}
	heads := make([]*cleanupNode, 0)
	for i := range rows {
		node := nodes[rows[i].ID]
		parent := listParent(nodes, node)
		if parent != nil {
			parent.children = append(parent.children, node)
		}
		// A thread starts at an archived session whose list parent is gone or
		// still in the list; a subagent always follows its parent.
		if node.archived() && !node.subagent() && (parent == nil || !parent.archived()) {
			heads = append(heads, node)
		}
	}
	for _, node := range nodes {
		slices.SortFunc(node.children, func(a, b *cleanupNode) int { return cmp.Compare(a.row.ID, b.row.ID) })
	}
	slices.SortFunc(heads, func(a, b *cleanupNode) int { return cmp.Compare(a.row.ID, b.row.ID) })
	return heads
}

func listParent(nodes map[string]*cleanupNode, node *cleanupNode) *cleanupNode {
	parentID := strings.TrimSpace(node.row.ParentSessionID)
	if parentID == "" || parentID == node.row.ID {
		return nil
	}
	switch node.row.LineageKind {
	case store.LineageKindContinue, store.LineageKindFork:
		return nil
	}
	parent := nodes[parentID]
	if parent == nil || parent.row.WorkspaceID != node.row.WorkspaceID {
		return nil
	}
	return parent
}

// pickThreads keeps the threads in range: archived strictly before the cutoff,
// limited to RootIDs when they are given, never an always-on agent's main session.
func pickThreads(heads []*cleanupNode, request Request) []*cleanupNode {
	var only map[string]struct{}
	if len(request.RootIDs) > 0 {
		only = make(map[string]struct{}, len(request.RootIDs))
		for _, id := range request.RootIDs {
			only[strings.TrimSpace(id)] = struct{}{}
		}
	}
	picked := make([]*cleanupNode, 0, len(heads))
	for _, head := range heads {
		if head.row.InstanceMain {
			continue
		}
		if request.ArchivedBefore != nil && !head.row.ArchivedAt.Before(*request.ArchivedBefore) {
			continue
		}
		if only != nil {
			if _, ok := only[head.row.ID]; !ok {
				continue
			}
		}
		picked = append(picked, head)
	}
	return picked
}

func planCounts(roots []*cleanupNode, selection Selection) Counts {
	var counts Counts
	for _, root := range roots {
		thread, _ := planThread(root, selection)
		counts.add(thread.Counts)
	}
	return counts
}

// planThread decides which sessions of one thread go. A session goes when the
// selection names its kind or its parent goes; a session still in the list or
// in Later never goes, and nor does anything under it.
func planThread(root *cleanupNode, selection Selection) (Thread, int) {
	thread := Thread{RootID: root.row.ID, WorkspaceID: root.row.WorkspaceID, Title: root.row.Name}
	kept := 0
	var walk func(node *cleanupNode, deleted bool)
	walk = func(node *cleanupNode, deleted bool) {
		for _, child := range node.children {
			goes := deleted || (child.subagent() && selection.Subagents) ||
				(!child.subagent() && selection.Children)
			if !child.archived() || child.row.InstanceMain {
				if goes {
					kept++
				}
				continue
			}
			walk(child, goes)
			if !goes {
				continue
			}
			thread.Delete = append(thread.Delete, child.row.ID)
			if child.subagent() {
				thread.Counts.Subagents++
			} else {
				thread.Counts.Children++
			}
		}
	}
	walk(root, selection.Everything)
	if selection.Everything {
		thread.Delete = append(thread.Delete, root.row.ID)
		thread.Counts.Sessions++
	}
	return thread, kept
}
