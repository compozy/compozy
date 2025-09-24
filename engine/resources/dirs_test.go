package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirForType_KnownTypes(t *testing.T) {
	// Map of types to expected non-empty dir result
	cases := []ResourceType{
		ResourceWorkflow,
		ResourceAgent,
		ResourceTool,
		ResourceTask,
		ResourceSchema,
		ResourceMCP,
		ResourceModel,
		ResourceMemory,
		ResourceProject,
	}
	for _, typ := range cases {
		dir, ok := DirForType(typ)
		assert.True(t, ok, "expected ok for %v", typ)
		assert.NotEmpty(t, dir, "dir should not be empty for %v", typ)
	}
}

func TestDirForType_Unknown(t *testing.T) {
	// Unknown types should return ok=false and empty dir
	dir, ok := DirForType("unknown-type")
	assert.False(t, ok)
	assert.Empty(t, dir)
}
