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
	total := len(items)
	start, end := computeWindow(items, cursorValue, direction, limit)
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
		nextValue = window[len(window)-1].Key.ID
		nextDir = CursorDirectionAfter
	}
	if len(window) > 0 && start > 0 {
		prevValue = window[0].Key.ID
		prevDir = CursorDirectionBefore
	}
	return window, nextValue, nextDir, prevValue, prevDir
}

func computeWindow(
	items []resources.StoredItem,
	cursorValue string,
	direction CursorDirection,
	limit int,
) (int, int) {
	switch direction {
	case CursorDirectionAfter:
		start := searchAfter(items, cursorValue)
		end := start + limit
		if end > len(items) {
			end = len(items)
		}
		return start, end
	case CursorDirectionBefore:
		end := searchBefore(items, cursorValue)
		if end < 0 {
			end = 0
		}
		start := end - limit
		if start < 0 {
			start = 0
		}
		return start, end
	default:
		end := limit
		if end > len(items) {
			end = len(items)
		}
		return 0, end
	}
}

func searchAfter(items []resources.StoredItem, value string) int {
	if value == "" {
		return 0
	}
	return sort.Search(len(items), func(i int) bool { return items[i].Key.ID > value })
}

func searchBefore(items []resources.StoredItem, value string) int {
	if value == "" {
		return len(items)
	}
	idx := sort.Search(len(items), func(i int) bool { return items[i].Key.ID >= value })
	return idx
}
