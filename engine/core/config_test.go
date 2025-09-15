package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_AsMapDefault_And_FromMapDefault(t *testing.T) {
	t.Run("Should convert struct to map using AsMapDefault", func(t *testing.T) {
		type cfg struct {
			A string
			B int
		}
		m, err := AsMapDefault(cfg{A: "x", B: 2})
		require.NoError(t, err)
		assert.Equal(t, "x", m["A"])
		assert.Equal(t, float64(2), m["B"]) // json encodes numbers as float64
	})
	t.Run("Should decode map into struct using FromMapDefault with weak types", func(t *testing.T) {
		type cfg struct {
			A string `mapstructure:"a"`
			B int    `mapstructure:"b"`
		}
		in := map[string]any{"a": "hello", "b": "42"}
		got, err := FromMapDefault[cfg](in)
		require.NoError(t, err)
		assert.Equal(t, "hello", got.A)
		assert.Equal(t, 42, got.B)
	})
}
