package webhook

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadRawJSON(t *testing.T) {
	t.Run("Should accept valid JSON within limit", func(t *testing.T) {
		r := bytes.NewBufferString("{\"a\":1}")
		b, err := ReadRawJSON(r, 1024)
		assert.NoError(t, err)
		assert.Equal(t, []byte("{\"a\":1}"), b)
	})

	t.Run("Should reject oversized body", func(t *testing.T) {
		large := "{\"k\":\"" + strings.Repeat("x", 2048) + "\"}"
		r := strings.NewReader(large)
		b, err := ReadRawJSON(r, 1024)
		assert.Nil(t, b)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "payload too large")
	})

	t.Run("Should reject invalid JSON", func(t *testing.T) {
		r := strings.NewReader("not-json")
		b, err := ReadRawJSON(r, 1024)
		assert.Nil(t, b)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid json")
	})
}
