package orchestrator

import (
	"encoding/json"
	"testing"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/assert"
)

func TestStableJSONFingerprint(t *testing.T) {
	t.Run("Should be identical for key order changes", func(t *testing.T) {
		a := []byte(`{"a":1,"b":2}`)
		b := []byte(`{"b":2,"a":1}`)
		assert.Equal(t, stableJSONFingerprint(a), stableJSONFingerprint(b))
	})
	t.Run("Should hash raw when invalid JSON", func(t *testing.T) {
		fp1 := stableJSONFingerprint([]byte("not-json"))
		fp2 := stableJSONFingerprint([]byte("not-json-2"))
		assert.NotEqual(t, fp1, fp2)
	})
}

func TestBuildIterationFingerprint_AndNoProgress(t *testing.T) {
	calls := []llmadapter.ToolCall{{ID: "1", Name: "t", Arguments: json.RawMessage(`{"x":1}`)}}
	results := []llmadapter.ToolResult{
		{ID: "1", Name: "t", Content: `{"ok":true}`, JSONContent: json.RawMessage(`{"ok":true}`)},
	}
	fp := buildIterationFingerprint(calls, results)
	assert.NotEmpty(t, fp)
	st := newLoopState(&settings{}, nil, nil)
	assert.False(t, st.detectNoProgress(2, fp))
	assert.False(t, st.detectNoProgress(2, fp))
	assert.True(t, st.detectNoProgress(2, fp))
}
