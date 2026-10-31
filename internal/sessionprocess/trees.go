package sessionprocess

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Tree roles: the lead names the tree, wrappers launched it, helpers run beside or below it.
const (
	TreeRoleLead    = "lead"
	TreeRoleWrapper = "wrapper"
	TreeRoleHelper  = "helper"
)

// Tree is one logical thing a session started: a dev server behind `bun run`, a browser with its
// helpers, a Terminal. Stopping it signals every member; its kind is the root's kind.
type Tree struct {
	Root, Lead Identity
	SessionID  string
	Kind       string
	// Wrappers run above Lead, outermost first; Members lists every process, Root first.
	Wrappers []Identity
	Members  []Identity
	Ports    []int
}

// assignTrees groups each session's rows into trees in place. A row joins the tree of its nearest
// attributed ancestor in the same session, then of its process-group leader. A detached root
// (re-parented to init) whose executable shares an app bundle or private directory with an earlier
// tree folds into it, as Chrome's crashpad handlers do.
func assignTrees(rows []Process) {
	bySession := make(map[string][]int)
	for i := range rows {
		bySession[rows[i].Owner.SessionID] = append(bySession[rows[i].Owner.SessionID], i)
	}
	for _, indexes := range bySession {
		assignSessionTrees(rows, indexes)
	}
}

func assignSessionTrees(rows []Process, indexes []int) {
	slices.SortFunc(indexes, func(a, b int) int { return compareStart(rows[a], rows[b]) })
	byPID := make(map[int]int, len(indexes))
	for _, i := range indexes {
		byPID[rows[i].PID] = i
	}
	parent := make(map[int]int, len(indexes))
	for _, i := range indexes {
		if p, ok := byPID[rows[i].PPID]; ok && p != i && !rows[p].StartedAt.After(rows[i].StartedAt) {
			parent[i] = p
		}
	}
	root := func(i int) int {
		seen := map[int]bool{}
		for !seen[i] {
			seen[i] = true
			p, ok := parent[i]
			if !ok {
				return i
			}
			i = p
		}
		return i
	}
	joinProcessGroups(rows, indexes, byPID, parent, root)
	foldDetachedHelpers(rows, indexes, parent, root)
	members := make(map[int][]int)
	for _, i := range indexes {
		r := root(i)
		members[r] = append(members[r], i)
	}
	for r, tree := range members {
		labelTree(rows, r, tree, parent)
	}
}

// helperWindow bounds how long after its app a detached helper starts: Chrome's crash handler
// starts within a second of the browser.
const helperWindow = 5 * time.Second

// joinProcessGroups attaches a root still in another row's process group to that group's leader,
// or to the earliest root of a leaderless group, when both are the same kind: background work
// re-parented out of a tool call stays left running, never hidden in the turn's tool tree.
func joinProcessGroups(rows []Process, indexes []int, byPID, parent map[int]int, root func(int) int) {
	sameKind := func(i, other int) bool { return rows[root(other)].Kind == rows[i].Kind && root(other) != i }
	groupRoot := make(map[int]int)
	for _, i := range indexes {
		if _, ok := parent[i]; ok || rows[i].PGID <= 1 || rows[i].PGID == rows[i].PID {
			continue
		}
		if leader, ok := byPID[rows[i].PGID]; ok && rows[leader].PGID == rows[leader].PID &&
			!rows[leader].StartedAt.After(rows[i].StartedAt) && sameKind(i, leader) {
			parent[i] = leader
			continue
		}
		if first, ok := groupRoot[rows[i].PGID]; ok && sameKind(i, first) {
			parent[i] = first
			continue
		}
		if _, ok := groupRoot[rows[i].PGID]; !ok {
			groupRoot[rows[i].PGID] = i
		}
	}
}

// foldDetachedHelpers folds a detached root (re-parented to init) into the tree that most recently
// started a process from the same install (app bundle or private directory), so each browser
// instance keeps its own crash handlers. Only a different program from that install, started within
// helperWindow, counts as a helper: two daemonized copies of one server stay separate trees, and a
// launcher such as a node script never folds because its interpreter lives in a shared bin directory.
func foldDetachedHelpers(rows []Process, indexes []int, parent map[int]int, root func(int) int) {
	owners := make(map[string][]int)
	for _, i := range indexes {
		key := bundleKey(rows[i].Argv0)
		_, attached := parent[i]
		if !attached && key != "" && rows[i].PPID <= 1 && rows[i].Kind == kindBackground {
			if owner, ok := latestOwner(rows, owners[key], i, root); ok {
				parent[i] = owner
			}
		}
		if key != "" && rows[root(i)].Kind != kindTool {
			owners[key] = append(owners[key], i)
		}
	}
}

// latestOwner picks the last process started from the same install before the detached root, in a
// tree other than the root's own, running a different program; it counts only when it started
// shortly before the root.
func latestOwner(rows []Process, candidates []int, detached int, root func(int) int) (int, bool) {
	for _, owner := range slices.Backward(candidates) {
		// Another copy of the same program is a peer, not the app this helper serves.
		// A tree rooted in the same program is another instance, reached through its own helper.
		if root(owner) == detached || rows[owner].Argv0 == rows[detached].Argv0 ||
			rows[root(owner)].Argv0 == rows[detached].Argv0 ||
			rows[owner].StartedAt.After(rows[detached].StartedAt) {
			continue
		}
		return owner, rows[detached].StartedAt.Sub(rows[owner].StartedAt) <= helperWindow
	}
	return 0, false
}

// bundleKey names the install an executable belongs to: its macOS app bundle, else its directory
// unless that directory is a shared bin directory where unrelated programs live side by side.
func bundleKey(argv0 string) string {
	if !filepath.IsAbs(argv0) {
		return ""
	}
	if i := strings.Index(argv0, ".app/"); i >= 0 {
		return argv0[:i+len(".app")]
	}
	dir := filepath.Dir(argv0)
	switch filepath.Base(dir) {
	case "bin", "sbin", ".bin", "/":
		return ""
	}
	return dir
}

func labelTree(rows []Process, r int, tree []int, parent map[int]int) {
	children := make(map[int][]int)
	for _, i := range tree {
		if p, ok := parent[i]; ok {
			children[p] = append(children[p], i)
		}
	}
	lead := r
	var wrappers []int
	for isWrapper(rows[lead]) && len(children[lead]) == 1 {
		wrappers = append(wrappers, lead)
		lead = children[lead][0]
	}
	rootID := identity(rows[r].ScannedProcess)
	for _, i := range tree {
		rows[i].Tree = rootID
		rows[i].TreeRole = TreeRoleHelper
	}
	for _, i := range wrappers {
		rows[i].TreeRole = TreeRoleWrapper
	}
	rows[lead].TreeRole = TreeRoleLead
}

// isWrapper reports launchers whose only job is to start one child: `sh -c`, `bun run`,
// `npm run`, `npx`. The tree is named by what they start.
func isWrapper(p Process) bool {
	fields := strings.Fields(p.Command)
	program := filepath.Base(cmp.Or(p.Argv0, firstField(fields)))
	args := fields
	if len(args) > 0 {
		args = args[1:]
	}
	switch program {
	case "sh", "bash", "zsh", "dash":
		return slices.Contains(args, "-c")
	case "npx", "bunx", "pnpx", "yarn":
		return true
	case "bun":
		return slices.Contains(args, "run") || slices.Contains(args, "x")
	case "npm", "pnpm":
		return slices.ContainsFunc(args, func(arg string) bool {
			switch arg {
			case "run", "run-script", "exec", "start", "test", "x", "dlx":
				return true
			}
			return false
		})
	}
	return false
}

func firstField(fields []string) string {
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func compareStart(a, b Process) int {
	return cmp.Or(a.StartedAt.Compare(b.StartedAt), cmp.Compare(a.PID, b.PID))
}

// Trees groups rows that assignTrees labeled, ordered by root start. Ports are the members' union.
func Trees(rows []Process) []Tree {
	index := make(map[Identity]int)
	var trees []Tree
	ordered := slices.Clone(rows)
	slices.SortFunc(ordered, compareStart)
	for _, p := range ordered {
		id := p.Tree
		if id.PID == 0 {
			id = identity(p.ScannedProcess)
		}
		i, ok := index[id]
		if !ok {
			i = len(trees)
			index[id] = i
			trees = append(trees, Tree{Root: id, Lead: id, SessionID: p.Owner.SessionID})
		}
		tree := &trees[i]
		member := identity(p.ScannedProcess)
		tree.Members = append(tree.Members, member)
		tree.Ports = append(tree.Ports, p.Ports...)
		if member == id {
			tree.Kind = p.Kind
		}
		switch p.TreeRole {
		case TreeRoleLead:
			tree.Lead = member
		case TreeRoleWrapper:
			tree.Wrappers = append(tree.Wrappers, member)
		}
	}
	for i := range trees {
		slices.Sort(trees[i].Ports)
		trees[i].Ports = slices.Compact(trees[i].Ports)
		if trees[i].Ports == nil {
			trees[i].Ports = []int{}
		}
		// The root starts first, so it already leads Members; keep it there even on a tied start.
		root := trees[i].Root
		trees[i].Members = append([]Identity{root}, slices.DeleteFunc(trees[i].Members, func(m Identity) bool {
			return m == root
		})...)
	}
	return trees
}
