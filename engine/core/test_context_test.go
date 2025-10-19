package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_TestContextExpectedOutputs(t *testing.T) {
	t.Run("Should store and retrieve expected outputs in context", func(t *testing.T) {
		ctx := t.Context()
		m := map[string]Output{"t1": {"k": "v"}}
		ctx = WithExpectedOutputs(ctx, m)
		got := ExpectedOutputsFromContext(ctx)
		assert.Equal(t, m, got)
	})
	t.Run("Should return nil when not present", func(t *testing.T) {
		assert.Nil(t, ExpectedOutputsFromContext(t.Context()))
	})
}
