package chunk

import (
	"strings"
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

func TestProcessorAdaptiveSettings(t *testing.T) {
	processor, err := NewProcessor(Settings{
		Strategy: StrategyRecursive,
		Size:     100,
		Overlap:  10,
	})
	require.NoError(t, err)

	shortText := strings.Repeat("z", 400)
	size, overlap := processor.effectiveChunkSettings(nil, shortText)
	assert.LessOrEqual(t, size, 100)
	assert.GreaterOrEqual(t, size, minAdaptiveChunkSize)
	assert.LessOrEqual(t, overlap, size)

	pdfMeta := map[string]any{"content_type": "application/pdf"}
	longText := strings.Repeat("a", 25000)
	pdfSize, pdfOverlap := processor.effectiveChunkSettings(pdfMeta, longText)
	assert.Greater(t, pdfSize, 100)
	assert.LessOrEqual(t, pdfSize, maxAdaptiveChunkSize)
	assert.GreaterOrEqual(t, pdfOverlap, pdfSize/6)

	transcriptMeta := map[string]any{"source_type": "transcript"}
	transcriptSize, transcriptOverlap := processor.effectiveChunkSettings(transcriptMeta, longText)
	assert.Less(t, transcriptSize, pdfSize)
	assert.GreaterOrEqual(t, transcriptOverlap, transcriptSize/4)
}
