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
	t.Run("Should handle edge cases and boundaries", func(t *testing.T) {
		// negatives
		assert.Equal(t, -3, AnyToInt(-3))
		assert.Equal(t, -3, AnyToInt(int64(-3)))
		assert.Equal(t, -3, AnyToInt(-3.7))
		// numeric string remains unsupported (locks current intent)
		assert.Equal(t, 0, AnyToInt("12"))
		// large int64 boundary maps to int via Go cast (platform-dependent width)
		var big int64 = 1 << 40
		assert.Equal(t, int(big), AnyToInt(big))
		var smallNeg int64 = -(1 << 40)
		assert.Equal(t, int(smallNeg), AnyToInt(smallNeg))
	})
}
