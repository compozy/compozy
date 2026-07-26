package devcycle

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/frontmatter"
	looppkg "github.com/compozy/agh/internal/loop"
	"gopkg.in/yaml.v3"
)

func orderCompozyTasks(
	nodes []compozyTaskGraphNode,
	edges []compozyTaskGraphEdge,
	files map[string]compozyTaskFile,
) ([]compozyTaskFile, error) {
	orderedIDs, err := compozyTaskTopologicalIDs(nodes, edges)
	if err != nil {
		return nil, err
	}
	ordered := make([]compozyTaskFile, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		taskFile, ok := files[id]
		if !ok {
			return nil, fmt.Errorf("%w: md_tasks node %q has no loaded file", looppkg.ErrValidation, id)
		}
		ordered = append(ordered, taskFile)
	}
	return ordered, nil
}

func compozyTaskTopologicalIDs(
	nodes []compozyTaskGraphNode,
	edges []compozyTaskGraphEdge,
) ([]string, error) {
	ids := make([]string, 0, len(nodes))
	predecessors := make(map[string]int, len(nodes))
	successors := make(map[string][]string, len(nodes))
	numbers := make(map[string]int, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
		predecessors[node.ID] = 0
		numbers[node.ID] = taskNumberFromTaskFile(node.File)
	}
	for _, edge := range edges {
		if _, ok := predecessors[edge.From]; !ok {
			continue
		}
		if _, ok := predecessors[edge.To]; !ok {
			continue
		}
		successors[edge.From] = append(successors[edge.From], edge.To)
		predecessors[edge.To]++
	}
	ready := make([]string, 0, len(ids))
	for _, id := range ids {
		if predecessors[id] == 0 {
			ready = append(ready, id)
		}
	}
	sortCompozyTaskIDs(ready, numbers)
	ordered := make([]string, 0, len(ids))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, id)
		sortCompozyTaskIDs(successors[id], numbers)
		for _, successor := range successors[id] {
			predecessors[successor]--
			if predecessors[successor] == 0 {
				ready = append(ready, successor)
			}
		}
		sortCompozyTaskIDs(ready, numbers)
	}
	if len(ordered) != len(ids) {
		remaining := make([]string, 0)
		for _, id := range ids {
			if predecessors[id] > 0 {
				remaining = append(remaining, id)
			}
		}
		sortCompozyTaskIDs(remaining, numbers)
		return nil, fmt.Errorf(
			"%w: md_tasks graph contains a cycle involving: %s",
			looppkg.ErrValidation,
			strings.Join(remaining, ", "),
		)
	}
	return ordered, nil
}

func sortCompozyTaskIDs(ids []string, numbers map[string]int) {
	slices.SortStableFunc(ids, func(a, b string) int {
		if numbers[a] != numbers[b] {
			return numbers[a] - numbers[b]
		}
		return strings.Compare(a, b)
	})
}

func compozyTaskBlocksByTarget(edges []compozyTaskGraphEdge) map[string][]string {
	blocks := make(map[string][]string)
	for _, edge := range edges {
		blocks[edge.To] = append(blocks[edge.To], edge.From)
	}
	for id := range blocks {
		slices.Sort(blocks[id])
	}
	return blocks
}

func compozyTaskBlocksForTarget(blocksByTarget map[string][]string, id string) []string {
	blocks := blocksByTarget[id]
	copied := make([]string, 0, len(blocks))
	return append(copied, blocks...)
}

func taskNumberFromTaskFile(file string) int {
	matches := taskFileNamePattern.FindStringSubmatch(pathBaseSlash(file))
	if len(matches) != 2 {
		return 0
	}
	number, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return number
}

func idFromTaskFile(file string) string {
	base := pathBaseSlash(file)
	if taskNumberFromTaskFile(base) <= 0 {
		return ""
	}
	return strings.TrimSuffix(base, ".md")
}

func pathBaseSlash(path string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
		return normalized[idx+1:]
	}
	return normalized
}

func compozyTaskStatusCompleted(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "done", "finished":
		return true
	default:
		return false
	}
}

func frontmatterHasKey(content []byte, key string) bool {
	parts, err := frontmatter.Split(content)
	if err != nil {
		return false
	}
	var node yaml.Node
	if err := yaml.Unmarshal(parts.Metadata, &node); err != nil {
		return false
	}
	mapping := &node
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return false
		}
		mapping = node.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return false
	}
	for idx := 0; idx+1 < len(mapping.Content); idx += 2 {
		keyNode := mapping.Content[idx]
		if keyNode.Kind == yaml.ScalarNode && strings.EqualFold(strings.TrimSpace(keyNode.Value), key) {
			return true
		}
	}
	return false
}
