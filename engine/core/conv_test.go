package core

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_ToStringMap(t *testing.T) {
	t.Run("Should copy map[string]string and avoid aliasing", func(t *testing.T) {
		src := map[string]string{"a": "1", "b": "2"}
		got := ToStringMap(src)
		assert.Equal(t, src, got)
		src["a"] = "x"
		assert.NotEqual(t, src["a"], got["a"]) // copy not alias
	})
	t.Run("Should extract only string values from map[string]any", func(t *testing.T) {
		src := map[string]any{"a": "1", "b": 2, "c": true}
		got := ToStringMap(src)
		assert.Equal(t, map[string]string{"a": "1"}, got)
	})
	t.Run("Should return nil for unsupported input", func(t *testing.T) {
		assert.Nil(t, ToStringMap(123))
	})
}

func Test_ParseAnyDuration(t *testing.T) {
	t.Run("Should parse human string", func(t *testing.T) {
		d, ok := ParseAnyDuration("1 hour")
		assert.True(t, ok)
		assert.Equal(t, time.Hour, d)
	})
	t.Run("Should parse numbers", func(t *testing.T) {
		d1, ok1 := ParseAnyDuration(5)
		d2, ok2 := ParseAnyDuration(int64(7))
		d3, ok3 := ParseAnyDuration(float64(9))
		assert.True(t, ok1 && ok2 && ok3)
		assert.Equal(t, time.Duration(5), d1)
		assert.Equal(t, time.Duration(7), d2)
		assert.Equal(t, time.Duration(9), d3)
	})
	t.Run("Should return false for empty/invalid", func(t *testing.T) {
		_, ok1 := ParseAnyDuration("")
		_, ok2 := ParseAnyDuration("nope")
		assert.False(t, ok1)
		assert.False(t, ok2)
	})
	t.Run("Should reject whitespace-only string", func(t *testing.T) {
		_, ok := ParseAnyDuration("   ")
		assert.False(t, ok)
	})
}

func Test_ParseAnyInt(t *testing.T) {
	t.Run("Should parse common numeric forms", func(t *testing.T) {
		i1, ok1 := ParseAnyInt(42)
		i2, ok2 := ParseAnyInt(int64(7))
		i3, ok3 := ParseAnyInt(float64(9))
		i4, ok4 := ParseAnyInt("10")
		n := json.Number("11")
		i5, ok5 := ParseAnyInt(n)
		assert.True(t, ok1 && ok2 && ok3 && ok4 && ok5)
		assert.Equal(t, 42, i1)
		assert.Equal(t, 7, i2)
		assert.Equal(t, 9, i3)
		assert.Equal(t, 10, i4)
		assert.Equal(t, 11, i5)
	})
	t.Run("Should reject non-integers and blanks", func(t *testing.T) {
		_, ok1 := ParseAnyInt(42.5)
		_, ok2 := ParseAnyInt(" ")
		_, ok3 := ParseAnyInt("abc")
		assert.False(t, ok1 || ok2 || ok3)
	})
	t.Run("Should reject decimal json.Number", func(t *testing.T) {
		_, ok := ParseAnyInt(json.Number("11.2"))
		assert.False(t, ok)
	})
}
