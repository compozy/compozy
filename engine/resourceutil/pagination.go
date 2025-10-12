package resourceutil

import (
	"sort"
	"strings"

	"github.com/compozy/compozy/engine/resources"
)

type CursorDirection string

const (
	CursorDirectionNone   CursorDirection = ""
	CursorDirectionAfter  CursorDirection = "after"
	CursorDirectionBefore CursorDirection = "before"
)

func ClampLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func FilterStoredItems(items []resources.StoredItem, prefix string) []resources.StoredItem {
	if prefix == "" {
		return sortStoredItems(items)
	}
	filtered := make([]resources.StoredItem, 0, len(items))
	for i := range items {
		if strings.HasPrefix(items[i].Key.ID, prefix) {
			filtered = append(filtered, items[i])
		}
	}
	return sortStoredItems(filtered)
}

func sortStoredItems(items []resources.StoredItem) []resources.StoredItem {
	if len(items) <= 1 {
		return items
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key.ID < items[j].Key.ID })
	return items
}

func ApplyCursorWindow(
	items []resources.StoredItem,
	cursorValue string,
	direction CursorDirection,
	limit int,
) ([]resources.StoredItem, string, CursorDirection, string, CursorDirection) {
	return applyCursorWindow(items, cursorValue, direction, limit, func(item resources.StoredItem) string {
		return item.Key.ID
	})
}

func ApplyCursorWindowIDs(
	ids []string,
	cursorValue string,
	direction CursorDirection,
	limit int,
) ([]string, string, CursorDirection, string, CursorDirection) {
	return applyCursorWindow(ids, cursorValue, direction, limit, func(id string) string { return id })
}

func applyCursorWindow[T any](
	items []T,
	cursorValue string,
	direction CursorDirection,
	limit int,
	idOf func(T) string,
) ([]T, string, CursorDirection, string, CursorDirection) {
	total := len(items)
	start, end := computeWindow(items, cursorValue, direction, limit, idOf)
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	window := items[start:end]
	var nextValue, prevValue string
	var nextDir, prevDir CursorDirection
	if len(window) > 0 && end < total {
		nextValue = idOf(window[len(window)-1])
		nextDir = CursorDirectionAfter
	}
	if len(window) > 0 && start > 0 {
		prevValue = idOf(window[0])
		prevDir = CursorDirectionBefore
	}
	return window, nextValue, nextDir, prevValue, prevDir
}

func computeWindow[T any](
	items []T,
	cursorValue string,
	direction CursorDirection,
	limit int,
	idOf func(T) string,
) (int, int) {
	switch direction {
	case CursorDirectionAfter:
		start := searchAfter(items, cursorValue, idOf)
		end := min(start+limit, len(items))
		return start, end
	case CursorDirectionBefore:
		end := max(searchBefore(items, cursorValue, idOf), 0)
		start := max(end-limit, 0)
		return start, end
	default:
		end := min(limit, len(items))
		return 0, end
	}
}

func searchAfter[T any](items []T, value string, idOf func(T) string) int {
	if value == "" {
		return 0
	}
	return sort.Search(len(items), func(i int) bool { return idOf(items[i]) > value })
}

func searchBefore[T any](items []T, value string, idOf func(T) string) int {
	if value == "" {
		return len(items)
	}
	return sort.Search(len(items), func(i int) bool { return idOf(items[i]) >= value })
}
