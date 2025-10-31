package compozy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractCycle(t *testing.T) {
	path := []string{"task:alpha/a", "task:alpha/b", "task:alpha/c"}
	cycle := extractCycle(path, "task:alpha/b")
	assert.Equal(t, []string{"task:alpha/b", "task:alpha/c", "task:alpha/b"}, cycle)
}

func TestParseNode(t *testing.T) {
	typ, id := parseNode("workflow:sample")
	assert.Equal(t, "workflow", typ)
	assert.Equal(t, "sample", id)
	typ, id = parseNode("invalidnode")
	assert.Equal(t, "invalidnode", typ)
	assert.Equal(t, "", id)
}
