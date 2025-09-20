package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestETag_Stability(t *testing.T) {
	t.Run("Should generate stable ETag for typed map[string]string", func(t *testing.T) {
		a := map[string]string{"b": "2", "a": "1", "c": "3"}
		b := map[string]string{"c": "3", "b": "2", "a": "1"}
		require.Equal(t, ETagFromAny(a), ETagFromAny(b))
	})
	t.Run("Should generate stable ETag for typed map[string]int", func(t *testing.T) {
		a := map[string]int{"x": 1, "y": 2}
		b := map[string]int{"y": 2, "x": 1}
		require.Equal(t, ETagFromAny(a), ETagFromAny(b))
	})
	t.Run("Should generate stable ETag for nested typed maps", func(t *testing.T) {
		a := map[string]map[string]string{"outer": {"b": "2", "a": "1"}}
		b := map[string]map[string]string{"outer": {"a": "1", "b": "2"}}
		require.Equal(t, ETagFromAny(a), ETagFromAny(b))
	})
}
