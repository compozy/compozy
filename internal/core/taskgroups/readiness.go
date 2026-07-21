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
		if !taskGroups[dependency.From].Completed {
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

func unmetTransitivePaths(plan Plan, taskGroups map[string]*TaskGroup, selected string) []DependencyPath {
	incoming := make(map[string][]Dependency, len(taskGroups))
	for _, edge := range plan.Edges {
		incoming[edge.To] = append(incoming[edge.To], edge)
	}
	for id := range incoming {
		slices.SortFunc(incoming[id], compareDependency)
	}
	paths := make(map[string]DependencyPath)
	var visit func(current string, edges []Dependency, ids []string, ancestors map[string]struct{})
	visit = func(current string, edges []Dependency, ids []string, ancestors map[string]struct{}) {
		for _, edge := range incoming[current] {
			if _, seen := ancestors[edge.From]; seen {
				continue
			}
			nextEdges := append(slices.Clone(edges), edge)
			nextIDs := append(slices.Clone(ids), edge.From)
			if len(nextEdges) > 1 && !taskGroups[edge.From].Completed {
				reversedIDs := reverseStrings(nextIDs)
				reversedEdges := reverseDependencies(nextEdges)
				reversedEdges = reversedEdges[:len(reversedEdges)-1]
				key := strings.Join(reversedIDs, "\x00")
				paths[key] = DependencyPath{TaskGroupIDs: reversedIDs, Edges: reversedEdges}
			}
			nextAncestors := make(map[string]struct{}, len(ancestors)+1)
			for id := range ancestors {
				nextAncestors[id] = struct{}{}
			}
			nextAncestors[edge.From] = struct{}{}
			visit(edge.From, nextEdges, nextIDs, nextAncestors)
		}
	}
	visit(selected, nil, []string{selected}, map[string]struct{}{selected: {}})
	result := make([]DependencyPath, 0, len(paths))
	for _, path := range paths {
		result = append(result, path)
	}
	slices.SortFunc(result, compareDependencyPath)
	return result
}

func reverseStrings(values []string) []string {
	result := make([]string, len(values))
	for index := range values {
		result[len(values)-1-index] = values[index]
	}
	if len(result) > 0 {
		result = result[:len(result)-1]
	}
	return result
}

func reverseDependencies(values []Dependency) []Dependency {
	result := make([]Dependency, len(values))
	for index := range values {
		result[len(values)-1-index] = values[index]
	}
	return result
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
