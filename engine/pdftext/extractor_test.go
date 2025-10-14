package pdftext

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractorProducesReadableText(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PDF extraction test in short mode")
	}
	extractor, err := New(Config{})
	require.NoError(t, err)
	t.Cleanup(func() { extractor.Close() })

	path := filepath.Join("..", "..", "test", "fixtures", "pdf", "indexing_optimization.pdf")
	result, err := extractor.ExtractFile(context.Background(), path, 0)
	require.NoError(t, err)
	require.True(t, result.Stats.IsReadable(), "expected readable text, issues: %v", result.Stats.Issues())
	require.Contains(t, result.Text, "Indexing Optimization")
}
