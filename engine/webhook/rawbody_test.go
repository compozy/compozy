package webhook

import (
	"bytes"
	"fmt"
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

	t.Run("Should accept body exactly at limit", func(t *testing.T) {
		// Create a JSON payload that's exactly 1024 bytes
		// {"data":"xxx..."} = 10 chars + content + 1 char = 11 + content
		// So content should be 1024 - 11 = 1013 chars
		payload := strings.Repeat("x", 1013)
		jsonPayload := fmt.Sprintf("{\"data\":%q}", payload)
		assert.Equal(t, 1024, len(jsonPayload), "Test payload should be exactly 1024 bytes")
		r := strings.NewReader(jsonPayload)
		b, err := ReadRawJSON(r, 1024)
		assert.NoError(t, err)
		assert.Equal(t, []byte(jsonPayload), b)
	})

	t.Run("Should handle empty object", func(t *testing.T) {
		r := strings.NewReader("{}")
		b, err := ReadRawJSON(r, 1024)
		assert.NoError(t, err)
		assert.Equal(t, []byte("{}"), b)
	})

	t.Run("Should handle null JSON", func(t *testing.T) {
		r := strings.NewReader("null")
		b, err := ReadRawJSON(r, 1024)
		assert.NoError(t, err)
		assert.Equal(t, []byte("null"), b)
	})

	t.Run("Should reject negative limit", func(t *testing.T) {
		r := strings.NewReader("{}")
		b, err := ReadRawJSON(r, -1)
		assert.Nil(t, b)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid limit")
	})
}
