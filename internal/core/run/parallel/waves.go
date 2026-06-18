package parallelrun

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/tasks"
)

// TaskID is the extensionless task identity used in task dependencies.
type TaskID string

// Waves is a deterministic topological layering of task ids.
type Waves struct {
	levels     [][]TaskID
	successors map[TaskID][]TaskID
}

// ErrCyclicDependencies identifies a task dependency cycle.
var ErrCyclicDependencies = errors.New("cyclic task dependencies")

// ErrMissingDependency identifies a dependency that references an absent task.
var ErrMissingDependency = errors.New("missing task dependency")

// CyclicDependenciesError reports the task ids participating in a dependency cycle.
type CyclicDependenciesError struct {
	Nodes []TaskID
}

func (e *CyclicDependenciesError) Error() string {
	if e == nil {
		return ErrCyclicDependencies.Error()
	}
	return fmt.Sprintf("%s: %s", ErrCyclicDependencies, joinTaskIDs(e.Nodes))
}

// Unwrap lets errors.Is match ErrCyclicDependencies.
func (e *CyclicDependenciesError) Unwrap() error {
	return ErrCyclicDependencies
}

// MissingDependencyError reports a dependency edge whose source task is absent.
type MissingDependencyError struct {
	TaskID     TaskID
	Dependency TaskID
}

func (e *MissingDependencyError) Error() string {
	if e == nil {
		return ErrMissingDependency.Error()
	}
	return fmt.Sprintf("%s: task %q depends on %q", ErrMissingDependency, e.TaskID, e.Dependency)
}

// Unwrap lets errors.Is match ErrMissingDependency.
func (e *MissingDependencyError) Unwrap() error {
	return ErrMissingDependency
}

// BuildWaves groups tasks into dependency-safe topological levels.
func BuildWaves(entries []model.TaskEntry) (Waves, error) {
	graph, err := buildGraph(entries)
	if err != nil {
		return Waves{}, err
	}
	return graph.waves()
}

// Levels returns the ordered waves. The returned slices are defensive copies.
func (w Waves) Levels() [][]TaskID {
	levels := make([][]TaskID, 0, len(w.levels))
	for _, level := range w.levels {
		levels = append(levels, slices.Clone(level))
	}
	return levels
}

// Len returns the number of waves.
func (w Waves) Len() int {
	return len(w.levels)
}

// BlockedBy returns every transitive dependent of the failed tasks mapped to
// the failed task that blocks it.
func (w Waves) BlockedBy(failed map[TaskID]bool) map[TaskID]TaskID {
	if len(failed) == 0 {
		return nil
	}
	blocked := make(map[TaskID]TaskID)
	roots := make([]TaskID, 0, len(failed))
	for id, isFailed := range failed {
		if isFailed {
			roots = append(roots, id)
		}
	}
	sortTaskIDs(roots)
	for _, root := range roots {
		w.markBlocked(root, root, failed, blocked, map[TaskID]struct{}{})
	}
	if len(blocked) == 0 {
		return nil
	}
	return blocked
}

func (w Waves) markBlocked(
	root TaskID,
	current TaskID,
	failed map[TaskID]bool,
	blocked map[TaskID]TaskID,
	visited map[TaskID]struct{},
) {
	for _, next := range w.successors[current] {
		if _, seen := visited[next]; seen {
			continue
		}
		visited[next] = struct{}{}
		nextRoot := root
		if failed[next] {
			nextRoot = next
		} else {
			if _, seen := blocked[next]; !seen {
				blocked[next] = root
			}
		}
		w.markBlocked(nextRoot, next, failed, blocked, visited)
	}
}

type dependencyGraph struct {
	ids          []TaskID
	predecessors map[TaskID]map[TaskID]struct{}
	successors   map[TaskID]map[TaskID]struct{}
}

func buildGraph(entries []model.TaskEntry) (*dependencyGraph, error) {
	graph := &dependencyGraph{
		ids:          make([]TaskID, 0, len(entries)),
		predecessors: make(map[TaskID]map[TaskID]struct{}, len(entries)),
		successors:   make(map[TaskID]map[TaskID]struct{}, len(entries)),
	}
	for idx := range entries {
		id := normalizeTaskID(entries[idx].ID)
		if id == "" {
			return nil, fmt.Errorf("task entry at index %d is missing task id", idx)
		}
		if _, exists := graph.predecessors[id]; exists {
			return nil, fmt.Errorf("duplicate task id %q", id)
		}
		graph.ids = append(graph.ids, id)
		graph.predecessors[id] = map[TaskID]struct{}{}
		graph.successors[id] = map[TaskID]struct{}{}
	}
	sortTaskIDs(graph.ids)

	for idx := range entries {
		taskID := normalizeTaskID(entries[idx].ID)
		for _, rawDependency := range entries[idx].Dependencies {
			dependency := normalizeTaskID(rawDependency)
			if dependency == "" {
				continue
			}
			if _, exists := graph.predecessors[dependency]; !exists {
				return nil, &MissingDependencyError{TaskID: taskID, Dependency: dependency}
			}
			graph.predecessors[taskID][dependency] = struct{}{}
			graph.successors[dependency][taskID] = struct{}{}
		}
	}
	return graph, nil
}

func (g *dependencyGraph) waves() (Waves, error) {
	if len(g.ids) == 0 {
		return Waves{}, nil
	}

	indegree := make(map[TaskID]int, len(g.ids))
	depth := make(map[TaskID]int, len(g.ids))
	ready := make([]TaskID, 0, len(g.ids))
	for _, id := range g.ids {
		indegree[id] = len(g.predecessors[id])
		if indegree[id] == 0 {
			ready = append(ready, id)
		}
	}

	ordered := make([]TaskID, 0, len(g.ids))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, id)

		successors := sortedSet(g.successors[id])
		for _, successor := range successors {
			if depth[id]+1 > depth[successor] {
				depth[successor] = depth[id] + 1
			}
			indegree[successor]--
			if indegree[successor] == 0 {
				ready = append(ready, successor)
				sortTaskIDs(ready)
			}
		}
	}

	if len(ordered) != len(g.ids) {
		return Waves{}, &CyclicDependenciesError{Nodes: g.cyclicNodes(indegree)}
	}

	maxDepth := 0
	for _, id := range ordered {
		if depth[id] > maxDepth {
			maxDepth = depth[id]
		}
	}
	levels := make([][]TaskID, maxDepth+1)
	for _, id := range ordered {
		levels[depth[id]] = append(levels[depth[id]], id)
	}
	for idx := range levels {
		sortTaskIDs(levels[idx])
	}
	return Waves{
		levels:     levels,
		successors: cloneSuccessors(g.successors),
	}, nil
}

func (g *dependencyGraph) cyclicNodes(indegree map[TaskID]int) []TaskID {
	remaining := make(map[TaskID]struct{})
	for _, id := range g.ids {
		if indegree[id] > 0 {
			remaining[id] = struct{}{}
		}
	}

	finder := cycleFinder{
		graph:     g,
		remaining: remaining,
		indexByID: map[TaskID]int{},
		lowByID:   map[TaskID]int{},
		onStack:   map[TaskID]bool{},
		cyclic:    map[TaskID]struct{}{},
	}
	for _, id := range g.ids {
		if _, ok := remaining[id]; !ok {
			continue
		}
		if _, seen := finder.indexByID[id]; seen {
			continue
		}
		finder.connect(id)
	}

	nodes := make([]TaskID, 0, len(finder.cyclic))
	for id := range finder.cyclic {
		nodes = append(nodes, id)
	}
	if len(nodes) == 0 {
		nodes = make([]TaskID, 0, len(remaining))
		for id := range remaining {
			nodes = append(nodes, id)
		}
	}
	sortTaskIDs(nodes)
	return nodes
}

type cycleFinder struct {
	graph     *dependencyGraph
	remaining map[TaskID]struct{}
	index     int
	indexByID map[TaskID]int
	lowByID   map[TaskID]int
	stack     []TaskID
	onStack   map[TaskID]bool
	cyclic    map[TaskID]struct{}
}

func (f *cycleFinder) connect(id TaskID) {
	f.indexByID[id] = f.index
	f.lowByID[id] = f.index
	f.index++
	f.stack = append(f.stack, id)
	f.onStack[id] = true

	for _, successor := range sortedSet(f.graph.successors[id]) {
		if _, ok := f.remaining[successor]; !ok {
			continue
		}
		if _, seen := f.indexByID[successor]; !seen {
			f.connect(successor)
			f.lowByID[id] = min(f.lowByID[id], f.lowByID[successor])
			continue
		}
		if f.onStack[successor] {
			f.lowByID[id] = min(f.lowByID[id], f.indexByID[successor])
		}
	}

	if f.lowByID[id] != f.indexByID[id] {
		return
	}

	component := f.popComponent(id)
	if len(component) > 1 || f.hasSelfLoop(id) {
		for _, node := range component {
			f.cyclic[node] = struct{}{}
		}
	}
}

func (f *cycleFinder) popComponent(root TaskID) []TaskID {
	component := make([]TaskID, 0)
	for len(f.stack) > 0 {
		last := len(f.stack) - 1
		node := f.stack[last]
		f.stack = f.stack[:last]
		f.onStack[node] = false
		component = append(component, node)
		if node == root {
			break
		}
	}
	return component
}

func (f *cycleFinder) hasSelfLoop(id TaskID) bool {
	_, ok := f.graph.successors[id][id]
	return ok
}

func normalizeTaskID(value string) TaskID {
	trimmed := strings.TrimSpace(strings.ReplaceAll(value, `\`, "/"))
	if trimmed == "" || strings.EqualFold(trimmed, "none") {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(trimmed)))
	if clean == "." {
		return ""
	}
	return TaskID(strings.TrimSuffix(clean, filepath.Ext(clean)))
}

func cloneSuccessors(src map[TaskID]map[TaskID]struct{}) map[TaskID][]TaskID {
	cloned := make(map[TaskID][]TaskID, len(src))
	for id, successors := range src {
		cloned[id] = sortedSet(successors)
	}
	return cloned
}

func sortedSet(set map[TaskID]struct{}) []TaskID {
	values := make([]TaskID, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sortTaskIDs(values)
	return values
}

func sortTaskIDs(ids []TaskID) {
	slices.SortFunc(ids, compareTaskID)
}

func compareTaskID(a, b TaskID) int {
	aNumber := tasks.ExtractTaskIdentityNumber(string(a))
	bNumber := tasks.ExtractTaskIdentityNumber(string(b))
	switch {
	case aNumber > 0 && bNumber > 0 && aNumber != bNumber:
		return aNumber - bNumber
	case aNumber > 0 && bNumber == 0:
		return -1
	case aNumber == 0 && bNumber > 0:
		return 1
	default:
		return strings.Compare(string(a), string(b))
	}
}

func joinTaskIDs(ids []TaskID) string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, string(id))
	}
	return strings.Join(values, ", ")
}
