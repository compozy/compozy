package attachment

import (
	"path/filepath"
	"testing"

	"github.com/jung-kurt/gofpdf"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
)

func Test_ToContentPartsFromEffective_PDF_TextExtraction(t *testing.T) {
	t.Run("Should extract text from small PDF and return TextPart", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		pdfPath := filepath.Join(dir, "hello.pdf")

		// Generate a tiny PDF with visible text using gofpdf
		doc := gofpdf.New("P", "mm", "A4", "")
		doc.AddPage()
		doc.SetFont("Helvetica", "", 14)
		doc.Text(10, 10, "Hello PDF from test")
		require.NoError(t, doc.OutputFileAndClose(pdfPath))

		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)

		items := []EffectiveItem{{Att: &PDFAttachment{Path: "hello.pdf"}, CWD: cwd}}
		parts, cleanup, err := ToContentPartsFromEffective(t.Context(), items)
		if cleanup != nil {
			defer cleanup()
		}
		require.NoError(t, err)
		require.Len(t, parts, 1)
		tp, ok := parts[0].(llmadapter.TextPart)
		require.True(t, ok)
		require.Contains(t, tp.Text, "Hello PDF")
	})
}
