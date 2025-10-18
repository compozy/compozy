package resourceutil

import (
	"testing"

	"github.com/compozy/compozy/engine/resources"
	"github.com/stretchr/testify/assert"
)

func TestClampLimit(t *testing.T) {
	t.Run("Should return default limit for zero value", func(t *testing.T) {
		result := ClampLimit(0)
		assert.Equal(t, 50, result)
	})

	t.Run("Should return default limit for negative value", func(t *testing.T) {
		result := ClampLimit(-10)
		assert.Equal(t, 50, result)
	})

	t.Run("Should return max limit when exceeding maximum", func(t *testing.T) {
		result := ClampLimit(1000)
		assert.Equal(t, 500, result)
	})

	t.Run("Should return same value when within range", func(t *testing.T) {
		result := ClampLimit(100)
		assert.Equal(t, 100, result)
	})

	t.Run("Should return minimum valid limit", func(t *testing.T) {
		result := ClampLimit(1)
		assert.Equal(t, 1, result)
	})

	t.Run("Should return maximum valid limit", func(t *testing.T) {
		result := ClampLimit(500)
		assert.Equal(t, 500, result)
	})

	t.Run("Should handle edge case at max boundary", func(t *testing.T) {
		result := ClampLimit(501)
		assert.Equal(t, 500, result)
	})
}

func createStoredItem(id string) resources.StoredItem {
	return resources.StoredItem{
		Key: resources.ResourceKey{
			Project: "test-project",
			Type:    resources.ResourceWorkflow,
			ID:      id,
		},
		Value: nil,
	}
}

func TestFilterStoredItems(t *testing.T) {
	t.Run("Should return all items when no prefix", func(t *testing.T) {
		items := []resources.StoredItem{
			createStoredItem("item-3"),
			createStoredItem("item-1"),
			createStoredItem("item-2"),
		}
		result := FilterStoredItems(items, "")
		assert.Len(t, result, 3)
		assert.Equal(t, "item-1", result[0].Key.ID)
		assert.Equal(t, "item-2", result[1].Key.ID)
		assert.Equal(t, "item-3", result[2].Key.ID)
	})

	t.Run("Should filter and sort items by prefix", func(t *testing.T) {
		items := []resources.StoredItem{
			createStoredItem("user-3"),
			createStoredItem("admin-1"),
			createStoredItem("user-1"),
			createStoredItem("admin-2"),
		}
		result := FilterStoredItems(items, "user-")
		assert.Len(t, result, 2)
		assert.Equal(t, "user-1", result[0].Key.ID)
		assert.Equal(t, "user-3", result[1].Key.ID)
	})

	t.Run("Should return empty slice when no matches", func(t *testing.T) {
		items := []resources.StoredItem{
			createStoredItem("item-1"),
			createStoredItem("item-2"),
		}
		result := FilterStoredItems(items, "admin-")
		assert.Empty(t, result)
	})

	t.Run("Should handle empty input", func(t *testing.T) {
		result := FilterStoredItems([]resources.StoredItem{}, "prefix")
		assert.Empty(t, result)
	})

	t.Run("Should sort single item", func(t *testing.T) {
		items := []resources.StoredItem{createStoredItem("single")}
		result := FilterStoredItems(items, "")
		assert.Len(t, result, 1)
		assert.Equal(t, "single", result[0].Key.ID)
	})

	t.Run("Should filter with partial match prefix", func(t *testing.T) {
		items := []resources.StoredItem{
			createStoredItem("workflow-agent-1"),
			createStoredItem("workflow-task-1"),
			createStoredItem("task-1"),
		}
		result := FilterStoredItems(items, "workflow-")
		assert.Len(t, result, 2)
		assert.Equal(t, "workflow-agent-1", result[0].Key.ID)
		assert.Equal(t, "workflow-task-1", result[1].Key.ID)
	})
}

func TestApplyCursorWindow(t *testing.T) {
	items := []resources.StoredItem{
		createStoredItem("item-1"),
		createStoredItem("item-2"),
		createStoredItem("item-3"),
		createStoredItem("item-4"),
		createStoredItem("item-5"),
	}

	t.Run("Should return first page with no cursor", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindow(
			items,
			"",
			CursorDirectionNone,
			2,
		)
		assert.Len(t, window, 2)
		assert.Equal(t, "item-1", window[0].Key.ID)
		assert.Equal(t, "item-2", window[1].Key.ID)
		assert.Equal(t, "item-2", next)
		assert.Equal(t, CursorDirectionAfter, nextDir)
		assert.Empty(t, prev)
		assert.Equal(t, CursorDirection(""), prevDir)
	})

	t.Run("Should return page after cursor", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindow(
			items,
			"item-2",
			CursorDirectionAfter,
			2,
		)
		assert.Len(t, window, 2)
		assert.Equal(t, "item-3", window[0].Key.ID)
		assert.Equal(t, "item-4", window[1].Key.ID)
		assert.Equal(t, "item-4", next)
		assert.Equal(t, CursorDirectionAfter, nextDir)
		assert.Equal(t, "item-3", prev)
		assert.Equal(t, CursorDirectionBefore, prevDir)
	})

	t.Run("Should return page before cursor", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindow(
			items,
			"item-5",
			CursorDirectionBefore,
			2,
		)
		assert.Len(t, window, 2)
		assert.Equal(t, "item-3", window[0].Key.ID)
		assert.Equal(t, "item-4", window[1].Key.ID)
		assert.Equal(t, "item-4", next)
		assert.Equal(t, CursorDirectionAfter, nextDir)
		assert.Equal(t, "item-3", prev)
		assert.Equal(t, CursorDirectionBefore, prevDir)
	})

	t.Run("Should return last page with no next cursor", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindow(
			items,
			"item-4",
			CursorDirectionAfter,
			2,
		)
		assert.Len(t, window, 1)
		assert.Equal(t, "item-5", window[0].Key.ID)
		assert.Empty(t, next)
		assert.Equal(t, CursorDirection(""), nextDir)
		assert.Equal(t, "item-5", prev)
		assert.Equal(t, CursorDirectionBefore, prevDir)
	})

	t.Run("Should handle empty input", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindow(
			[]resources.StoredItem{},
			"",
			CursorDirectionNone,
			10,
		)
		assert.Empty(t, window)
		assert.Empty(t, next)
		assert.Equal(t, CursorDirection(""), nextDir)
		assert.Empty(t, prev)
		assert.Equal(t, CursorDirection(""), prevDir)
	})

	t.Run("Should handle limit exceeding total items", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindow(
			items,
			"",
			CursorDirectionNone,
			100,
		)
		assert.Len(t, window, 5)
		assert.Empty(t, next)
		assert.Equal(t, CursorDirection(""), nextDir)
		assert.Empty(t, prev)
		assert.Equal(t, CursorDirection(""), prevDir)
	})

	t.Run("Should handle cursor at boundary", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindow(
			items,
			"item-1",
			CursorDirectionAfter,
			3,
		)
		assert.Len(t, window, 3)
		assert.Equal(t, "item-2", window[0].Key.ID)
		assert.Equal(t, "item-4", next)
		assert.Equal(t, CursorDirectionAfter, nextDir)
		assert.Equal(t, "item-2", prev)
		assert.Equal(t, CursorDirectionBefore, prevDir)
	})
}

func TestApplyCursorWindowIDs(t *testing.T) {
	ids := []string{"id-1", "id-2", "id-3", "id-4", "id-5"}

	t.Run("Should return first page of IDs with no cursor", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindowIDs(
			ids,
			"",
			CursorDirectionNone,
			2,
		)
		assert.Len(t, window, 2)
		assert.Equal(t, "id-1", window[0])
		assert.Equal(t, "id-2", window[1])
		assert.Equal(t, "id-2", next)
		assert.Equal(t, CursorDirectionAfter, nextDir)
		assert.Empty(t, prev)
		assert.Equal(t, CursorDirection(""), prevDir)
	})

	t.Run("Should return page after cursor for IDs", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindowIDs(
			ids,
			"id-2",
			CursorDirectionAfter,
			2,
		)
		assert.Len(t, window, 2)
		assert.Equal(t, "id-3", window[0])
		assert.Equal(t, "id-4", window[1])
		assert.Equal(t, "id-4", next)
		assert.Equal(t, CursorDirectionAfter, nextDir)
		assert.Equal(t, "id-3", prev)
		assert.Equal(t, CursorDirectionBefore, prevDir)
	})

	t.Run("Should return page before cursor for IDs", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindowIDs(
			ids,
			"id-5",
			CursorDirectionBefore,
			2,
		)
		assert.Len(t, window, 2)
		assert.Equal(t, "id-3", window[0])
		assert.Equal(t, "id-4", window[1])
		assert.Equal(t, "id-4", next)
		assert.Equal(t, CursorDirectionAfter, nextDir)
		assert.Equal(t, "id-3", prev)
		assert.Equal(t, CursorDirectionBefore, prevDir)
	})

	t.Run("Should handle empty IDs", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindowIDs(
			[]string{},
			"",
			CursorDirectionNone,
			10,
		)
		assert.Empty(t, window)
		assert.Empty(t, next)
		assert.Equal(t, CursorDirection(""), nextDir)
		assert.Empty(t, prev)
		assert.Equal(t, CursorDirection(""), prevDir)
	})

	t.Run("Should handle single ID", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindowIDs(
			[]string{"single-id"},
			"",
			CursorDirectionNone,
			10,
		)
		assert.Len(t, window, 1)
		assert.Equal(t, "single-id", window[0])
		assert.Empty(t, next)
		assert.Equal(t, CursorDirection(""), nextDir)
		assert.Empty(t, prev)
		assert.Equal(t, CursorDirection(""), prevDir)
	})
}

func TestCursorDirection(t *testing.T) {
	t.Run("Should define correct direction constants", func(t *testing.T) {
		assert.Equal(t, CursorDirection(""), CursorDirectionNone)
		assert.Equal(t, CursorDirection("after"), CursorDirectionAfter)
		assert.Equal(t, CursorDirection("before"), CursorDirectionBefore)
	})
}

func TestApplyCursorWindow_EdgeCases(t *testing.T) {
	items := []resources.StoredItem{
		createStoredItem("a"),
		createStoredItem("b"),
		createStoredItem("c"),
	}

	t.Run("Should handle cursor beyond last item", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindow(
			items,
			"z",
			CursorDirectionAfter,
			10,
		)
		assert.Empty(t, window)
		assert.Empty(t, next)
		assert.Equal(t, CursorDirection(""), nextDir)
		assert.Empty(t, prev)
		assert.Equal(t, CursorDirection(""), prevDir)
	})

	t.Run("Should handle cursor before first item", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindow(
			items,
			"0",
			CursorDirectionBefore,
			10,
		)
		assert.Empty(t, window)
		assert.Empty(t, next)
		assert.Equal(t, CursorDirection(""), nextDir)
		assert.Empty(t, prev)
		assert.Equal(t, CursorDirection(""), prevDir)
	})

	t.Run("Should handle limit of 1", func(t *testing.T) {
		window, next, nextDir, prev, prevDir := ApplyCursorWindow(
			items,
			"",
			CursorDirectionNone,
			1,
		)
		assert.Len(t, window, 1)
		assert.Equal(t, "a", window[0].Key.ID)
		assert.Equal(t, "a", next)
		assert.Equal(t, CursorDirectionAfter, nextDir)
		assert.Empty(t, prev)
		assert.Equal(t, CursorDirection(""), prevDir)
	})
}
