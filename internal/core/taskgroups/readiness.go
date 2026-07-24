package taskgroups

import (
	"fmt"
	"slices"
	"strings"
)

// EvaluateReadiness evaluates direct, transitive, and independent task group state.
func EvaluateReadiness(plan Plan, taskGroupID string) (Readiness, error) {
	selected, found := plan.TaskGroup(taskGroupID)
	if !found {
		return Readiness{}, taskGroupNotFound(Ref{Initiative: plan.Initiative, TaskGroupID: taskGroupID}, plan)
	}
	taskGroups := make(map[string]*TaskGroup, len(plan.TaskGroups))
	for index := range plan.TaskGroups {
		taskGroup := &plan.TaskGroups[index]
		taskGroups[taskGroup.ID] = taskGroup
	}
	for _, edge := range plan.Edges {
		if _, exists := taskGroups[edge.From]; !exists {
			return Readiness{}, newError(
				ErrInvalidPlan,
				plan.Initiative,
				taskGroupID,
				plan.Path,
				[]Issue{{Field: "graph.edges", Message: fmt.Sprintf("unknown prerequisite %q", edge.From)}},
			)
		}
		if _, exists := taskGroups[edge.To]; !exists {
			return Readiness{}, newError(
				ErrInvalidPlan,
				plan.Initiative,
				taskGroupID,
				plan.Path,
				[]Issue{{Field: "graph.edges", Message: fmt.Sprintf("unknown consumer %q", edge.To)}},
			)
		}
	}

	direct := make([]Dependency, 0)
	for _, dependency := range selected.Dependencies {
		prerequisite, exists := taskGroups[dependency.From]
		if !exists {
			return Readiness{}, newError(
				ErrInvalidPlan,
				plan.Initiative,
				taskGroupID,
				plan.Path,
				[]Issue{{Field: "graph.edges", Message: fmt.Sprintf("unknown prerequisite %q", dependency.From)}},
			)
		}
		if !prerequisite.Completed {
			direct = append(direct, dependency)
		}
	}
	slices.SortFunc(direct, compareDependency)
	transitive := unmetTransitivePaths(plan, taskGroups, taskGroupID)
	return Readiness{
		Eligible:         len(direct) == 0 && len(transitive) == 0,
		DirectUnmet:      direct,
		TransitiveUnmet:  transitive,
		IndependentPeers: independentPeers(plan, taskGroups, taskGroupID),
	}, nil
}

// dependencyPathLink records the reverse-graph BFS predecessor used to
// reconstruct one representative path from the selected group to an unmet
// transitive ancestor.
type dependencyPathLink struct {
	edge Dependency
	from string
}

// unmetTransitivePaths returns one representative path per incomplete transitive
// ancestor of the selected group. A breadth-first walk over reverse edges keeps
// this O(V+E): an earlier revision enumerated every simple path, which is
// exponential in graph density (a plan where TG-k depends on every TG-j records
// 2^(N-1)-N paths) and could wedge the daemon list path. Direct prerequisites
// are surfaced separately via DirectUnmet, so only ancestors reached at depth
// >= 2 are recorded here. The union of blockers and the Eligible verdict are
// preserved: every incomplete reverse-reachable ancestor is still covered
// (directly or as a path endpoint).
func unmetTransitivePaths(plan Plan, taskGroups map[string]*TaskGroup, selected string) []DependencyPath {
	incoming := make(map[string][]Dependency, len(taskGroups))
	for _, edge := range plan.Edges {
		incoming[edge.To] = append(incoming[edge.To], edge)
	}
	for id := range incoming {
		slices.SortFunc(incoming[id], compareDependency)
	}
	directPrerequisites := make(map[string]struct{}, len(incoming[selected]))
	for _, edge := range incoming[selected] {
		directPrerequisites[edge.From] = struct{}{}
	}
	parents := make(map[string]dependencyPathLink, len(taskGroups))
	visited := map[string]struct{}{selected: {}}
	queue := []string{selected}
	paths := make([]DependencyPath, 0)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range incoming[current] {
			ancestor := edge.From
			if _, seen := visited[ancestor]; seen {
				continue
			}
			visited[ancestor] = struct{}{}
			parents[ancestor] = dependencyPathLink{edge: edge, from: current}
			queue = append(queue, ancestor)
			if _, isDirect := directPrerequisites[ancestor]; isDirect {
				continue
			}
			if taskGroups[ancestor].Completed {
				continue
			}
			paths = append(paths, buildDependencyPath(ancestor, selected, parents))
		}
	}
	slices.SortFunc(paths, compareDependencyPath)
	return paths
}

// buildDependencyPath reconstructs the reverse-BFS chain from an unmet ancestor
// back to (but excluding) the selected group. TaskGroupIDs runs from the deepest
// ancestor down to the selected group's direct prerequisite; Edges connects each
// consecutive pair, excluding the final edge into the selected group.
func buildDependencyPath(ancestor, selected string, parents map[string]dependencyPathLink) DependencyPath {
	ids := make([]string, 0)
	edges := make([]Dependency, 0)
	node := ancestor
	for node != selected {
		ids = append(ids, node)
		link := parents[node]
		if link.from != selected {
			edges = append(edges, link.edge)
		}
		node = link.from
	}
	return DependencyPath{TaskGroupIDs: ids, Edges: edges}
}

func compareDependencyPath(left, right DependencyPath) int {
	leftKey := strings.Join(left.TaskGroupIDs, "\x00")
	rightKey := strings.Join(right.TaskGroupIDs, "\x00")
	return strings.Compare(leftKey, rightKey)
}

func independentPeers(plan Plan, taskGroups map[string]*TaskGroup, selected string) []string {
	forward := reachable(plan.Edges, selected, false)
	backward := reachable(plan.Edges, selected, true)
	peers := make([]string, 0)
	for id := range taskGroups {
		if id == selected || taskGroups[id].Completed || forward[id] || backward[id] {
			continue
		}
		peers = append(peers, id)
	}
	slices.Sort(peers)
	return peers
}

func reachable(edges []Dependency, selected string, reverse bool) map[string]bool {
	adjacent := make(map[string][]string)
	for _, edge := range edges {
		from, to := edge.From, edge.To
		if reverse {
			from, to = to, from
		}
		adjacent[from] = append(adjacent[from], to)
	}
	seen := make(map[string]bool)
	queue := []string{selected}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacent[current] {
			if seen[next] {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	return seen
}
