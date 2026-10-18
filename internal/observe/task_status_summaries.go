package observe

import (
	"slices"
	"strings"

	taskpkg "github.com/compozy/compozy/internal/task"
)

func summarizeTasks(tasks []taskpkg.Summary) []TaskStatusTotal {
	counts := make(map[string]TaskStatusTotal)
	for idx := range tasks {
		item := &tasks[idx]
		key := string(
			item.Scope.Normalize(),
		) + "\x00" + string(
			item.Status.Normalize(),
		)
		current := counts[key]
		current.Scope = item.Scope.Normalize()
		current.Status = item.Status.Normalize()
		current.Count++
		counts[key] = current
	}
	rows := make([]TaskStatusTotal, 0, len(counts))
	for _, item := range counts {
		rows = append(rows, item)
	}
	slices.SortFunc(rows, func(left, right TaskStatusTotal) int {
		if cmp := strings.Compare(string(left.Scope), string(right.Scope)); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(string(left.Status), string(right.Status)); cmp != 0 {
			return cmp
		}
		return 0
	})
	return rows
}

func summarizeTaskOrigins(tasks []taskpkg.Summary) []TaskOriginTotal {
	counts := make(map[string]TaskOriginTotal)
	for idx := range tasks {
		item := &tasks[idx]
		key := string(item.Origin.Kind.Normalize())
		current := counts[key]
		current.OriginKind = item.Origin.Kind.Normalize()
		current.Count++
		counts[key] = current
	}
	rows := make([]TaskOriginTotal, 0, len(counts))
	for _, item := range counts {
		rows = append(rows, item)
	}
	slices.SortFunc(rows, func(left, right TaskOriginTotal) int {
		if cmp := strings.Compare(string(left.OriginKind), string(right.OriginKind)); cmp != 0 {
			return cmp
		}
		return 0
	})
	return rows
}
