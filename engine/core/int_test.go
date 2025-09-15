package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_AnyToInt(t *testing.T) {
	t.Run("Should convert common forms to int", func(t *testing.T) {
		assert.Equal(t, 0, AnyToInt(nil))
		assert.Equal(t, 5, AnyToInt(5))
		assert.Equal(t, 7, AnyToInt(int64(7)))
		assert.Equal(t, 9, AnyToInt(9.2))
		assert.Equal(t, 0, AnyToInt("x"))
	})
}
