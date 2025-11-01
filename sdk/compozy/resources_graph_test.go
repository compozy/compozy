package compozy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractCycle(t *testing.T) {
	t.Run("Should derive cycle from traversal history", func(t *testing.T) {
		path := []string{"task:alpha/a", "task:alpha/b", "task:alpha/c"}
		cycle := extractCycle(path, "task:alpha/b")
		assert.Equal(t, []string{"task:alpha/b", "task:alpha/c", "task:alpha/b"}, cycle)
	})
	t.Run("Should return target when path empty", func(t *testing.T) {
		cycle := extractCycle(nil, "task:alpha/b")
		assert.Equal(t, []string{"task:alpha/b"}, cycle)
	})
	t.Run("Should return target when not in path", func(t *testing.T) {
		path := []string{"task:alpha/a", "task:alpha/c"}
		cycle := extractCycle(path, "task:alpha/b")
		assert.Equal(t, []string{"task:alpha/b"}, cycle)
	})
	t.Run("Should handle target at start", func(t *testing.T) {
		path := []string{"task:alpha/b", "task:alpha/c"}
		cycle := extractCycle(path, "task:alpha/b")
		assert.Equal(t, []string{"task:alpha/b", "task:alpha/c", "task:alpha/b"}, cycle)
	})
	t.Run("Should handle target at end", func(t *testing.T) {
		path := []string{"task:alpha/a", "task:alpha/b"}
		cycle := extractCycle(path, "task:alpha/b")
		assert.Equal(t, []string{"task:alpha/b", "task:alpha/b"}, cycle)
	})
	t.Run("Should handle single element path", func(t *testing.T) {
		path := []string{"task:alpha/b"}
		cycle := extractCycle(path, "task:alpha/b")
		assert.Equal(t, []string{"task:alpha/b", "task:alpha/b"}, cycle)
	})
	t.Run("Should use latest occurrence when target repeats", func(t *testing.T) {
		path := []string{"task:alpha/a", "task:alpha/b", "task:alpha/c", "task:alpha/b"}
		cycle := extractCycle(path, "task:alpha/b")
		assert.Equal(t, []string{"task:alpha/b", "task:alpha/b"}, cycle)
	})
}

func TestParseNode(t *testing.T) {
	t.Run("Should split node identifier into type and id", func(t *testing.T) {
		typ, id := parseNode("workflow:sample")
		assert.Equal(t, "workflow", typ)
		assert.Equal(t, "sample", id)
	})
	t.Run("Should return type and empty id when delimiter missing", func(t *testing.T) {
		typ, id := parseNode("invalidnode")
		assert.Equal(t, "invalidnode", typ)
		assert.Equal(t, "", id)
	})
	t.Run("Should return empty parts when input empty", func(t *testing.T) {
		typ, id := parseNode("")
		assert.Equal(t, "", typ)
		assert.Equal(t, "", id)
	})
	t.Run("Should split only on first delimiter", func(t *testing.T) {
		typ, id := parseNode("workflow:sample:extra")
		assert.Equal(t, "workflow", typ)
		assert.Equal(t, "sample:extra", id)
	})
	t.Run("Should handle delimiter only input", func(t *testing.T) {
		typ, id := parseNode(":")
		assert.Equal(t, "", typ)
		assert.Equal(t, "", id)
	})
	t.Run("Should trim whitespace around type and id", func(t *testing.T) {
		typ, id := parseNode(" workflow: sample ")
		assert.Equal(t, "workflow", typ)
		assert.Equal(t, "sample", id)
	})
}
