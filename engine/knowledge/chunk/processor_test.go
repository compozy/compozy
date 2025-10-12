package chunk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessor(t *testing.T) {
	settings := Settings{
		Strategy:          StrategyRecursive,
		Size:              20,
		Overlap:           2,
		RemoveHTML:        true,
		Deduplicate:       true,
		NormalizeNewlines: true,
	}
	t.Run("ShouldChunkNormalizeAndDeduplicate", func(t *testing.T) {
		processor, err := NewProcessor(settings)
		require.NoError(t, err)
		chunks, err := processor.Process("kb1", []Document{
			{
				ID:   "doc1",
				Text: "<p>Hello world.<br>Second line.</p>",
				Metadata: map[string]any{
					"path": "doc1",
				},
			},
			{
				ID:   "doc2",
				Text: "<div>Hello world.\nThird line.</div>",
			},
		})
		require.NoError(t, err)
		require.NotEmpty(t, chunks)
		assert.Equal(t, chunks[0].Metadata["source_id"], "doc1")
		assert.NotEmpty(t, chunks[0].ID)
		assert.Equal(t, chunks[0].Hash, hashText(chunks[0].Text))
		assert.Equal(t, chunks[0].Metadata["chunk_index"], 0)
		ids := make(map[string]struct{})
		for _, chunk := range chunks {
			ids[chunk.ID] = struct{}{}
		}
		assert.Len(t, ids, len(chunks))
	})
}
