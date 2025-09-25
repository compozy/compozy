package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirForType_KnownTypes(t *testing.T) {
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
		tc := typ
		t.Run("Should return directory for "+string(tc), func(t *testing.T) {
			dir, ok := DirForType(tc)
			assert.True(t, ok, "expected ok for %v", tc)
			assert.NotEmpty(t, dir, "dir should not be empty for %v", tc)
		})
	}
}

func TestDirForType_Unknown(t *testing.T) {
	t.Run("Should return no directory for unknown type", func(t *testing.T) {
		dir, ok := DirForType("unknown-type")
		assert.False(t, ok)
		assert.Empty(t, dir)
	})
}
