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
		texts := make(map[string]struct{})
		for _, chunk := range chunks {
			ids[chunk.ID] = struct{}{}
			texts[chunk.Text] = struct{}{}
			assert.NotContains(t, chunk.Text, "<")
			assert.NotContains(t, chunk.Text, ">")
		}
		assert.Len(t, chunks, 2)
		assert.Len(t, texts, 2)
		assert.Contains(t, texts, "Hello world.Second line.")
		assert.Contains(t, texts, "Hello world.\nThird line.")
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

	t.Run("Should adapt for short text", func(t *testing.T) {
		shortText := strings.Repeat("z", 400)
		size, overlap := processor.effectiveChunkSettings(nil, shortText)
		assert.LessOrEqual(t, size, 100)
		assert.GreaterOrEqual(t, size, minAdaptiveChunkSize)
		assert.LessOrEqual(t, overlap, size)
	})

	t.Run("Should expand for PDFs", func(t *testing.T) {
		pdfMeta := map[string]any{"content_type": "application/pdf"}
		longText := strings.Repeat("a", 25000)
		pdfSize, pdfOverlap := processor.effectiveChunkSettings(pdfMeta, longText)
		assert.Greater(t, pdfSize, 100)
		assert.LessOrEqual(t, pdfSize, maxAdaptiveChunkSize)
		assert.GreaterOrEqual(t, pdfOverlap, pdfSize/6)
	})

	t.Run("Should reduce for transcripts vs PDFs", func(t *testing.T) {
		longText := strings.Repeat("a", 25000)
		pdfMeta := map[string]any{"content_type": "application/pdf"}
		transcriptMeta := map[string]any{"source_type": "transcript"}
		pdfSize, _ := processor.effectiveChunkSettings(pdfMeta, longText)
		transcriptSize, transcriptOverlap := processor.effectiveChunkSettings(transcriptMeta, longText)
		assert.Less(t, transcriptSize, pdfSize)
		assert.GreaterOrEqual(t, transcriptOverlap, transcriptSize/4)
	})
}

func TestClampOverlap(t *testing.T) {
	tests := []struct {
		name     string
		overlap  int
		size     int
		expected int
	}{
		{"Should return zero when overlap is negative", -5, 100, 0},
		{"Should return zero when overlap is zero", 0, 100, 0},
		{"Should preserve valid overlap within size limit", 20, 100, 20},
		{"Should apply 25% cap when overlap equals size", 100, 100, 25},
		{"Should apply 25% cap when overlap exceeds size", 150, 100, 25},
		{"Should return zero for small size with large overlap", 50, 4, 0},
		{"Should return zero for small size with equal overlap", 4, 4, 0},
		{"Should apply 25% cap for size 5 with equal overlap", 5, 5, 1},
		{"Should apply 25% cap for size 8 with exceeding overlap", 10, 8, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clampOverlap(tt.overlap, tt.size)
			assert.Equal(
				t,
				tt.expected,
				result,
				"clampOverlap(%d, %d) should return %d",
				tt.overlap,
				tt.size,
				tt.expected,
			)
		})
	}
}
