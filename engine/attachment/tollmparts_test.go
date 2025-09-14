package attachment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/require"
)

func Test_ToContentPartsFromEffective_ImageURLAndPath(t *testing.T) {
	t.Run("Should map image URL to ImageURLPart with detail", func(t *testing.T) {
		att := &ImageAttachment{
			baseAttachment: baseAttachment{MetaMap: map[string]any{"image_detail": "high"}},
			Source:         SourceURL,
			URL:            "https://example.com/p.png",
		}
		parts, cleanup, err := ToContentPartsFromEffective(context.Background(), []EffectiveItem{{Att: att, CWD: nil}})
		require.NoError(t, err)
		if cleanup != nil {
			cleanup()
		}
		require.Equal(t, 1, len(parts))
		img, ok := parts[0].(llmadapter.ImageURLPart)
		require.True(t, ok)
		require.Equal(t, "https://example.com/p.png", img.URL)
		require.Equal(t, "high", img.Detail)
	})

	t.Run("Should map image Path to BinaryPart with detected MIME", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "a.png")
		data := []byte{137, 80, 78, 71, 13, 10, 26, 10}
		require.NoError(t, os.WriteFile(f, data, 0o644))
		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)
		att := &ImageAttachment{Source: SourcePath, Path: "a.png"}
		parts, cleanup, err := ToContentPartsFromEffective(context.Background(), []EffectiveItem{{Att: att, CWD: cwd}})
		require.NoError(t, err)
		if cleanup != nil {
			cleanup()
		}
		require.Equal(t, 1, len(parts))
		bin, ok := parts[0].(llmadapter.BinaryPart)
		require.True(t, ok)
		require.Equal(t, "image/png", bin.MIMEType)
		require.GreaterOrEqual(t, len(bin.Data), 8)
	})
}

func Test_ToContentPartsFromEffective_IgnoresNonImage(t *testing.T) {
	t.Run("Should ignore non-image attachments in current phase", func(t *testing.T) {
		a := &AudioAttachment{Source: SourceURL, URL: "https://example.com/a.mp3"}
		v := &VideoAttachment{Source: SourceURL, URL: "https://example.com/v.mp4"}
		parts, cleanup, err := ToContentPartsFromEffective(context.Background(), []EffectiveItem{{Att: a}, {Att: v}})
		require.NoError(t, err)
		if cleanup != nil {
			cleanup()
		}
		require.Equal(t, 0, len(parts))
	})
}

func Test_ToContentPartsFromEffective_TextFile(t *testing.T) {
	t.Run("Should resolve text file to TextPart", func(t *testing.T) {
		tmpDir := t.TempDir()
		txtPath := filepath.Join(tmpDir, "test.txt")
		textContent := "This is a test file"
		require.NoError(t, os.WriteFile(txtPath, []byte(textContent), 0644))
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		items := []EffectiveItem{{Att: &FileAttachment{baseAttachment: baseAttachment{}, Path: "test.txt"}, CWD: cwd}}
		parts, cleanup, err := ToContentPartsFromEffective(context.Background(), items)
		if cleanup != nil {
			cleanup()
		}
		require.NoError(t, err)
		require.Len(t, parts, 1)
		tp, ok := parts[0].(llmadapter.TextPart)
		require.True(t, ok)
		require.Equal(t, textContent, tp.Text)
	})
}

func Test_ToContentPartsFromEffective_PDF_FallbackBinary(t *testing.T) {
	t.Run("Should fallback to BinaryPart when extraction fails", func(t *testing.T) {
		dir := t.TempDir()
		pdfPath := filepath.Join(dir, "a.pdf")
		// Minimal PDF header to satisfy MIME detection
		header := []byte{'%', 'P', 'D', 'F', '-', '1', '.', '4', '\n'}
		require.NoError(t, os.WriteFile(pdfPath, header, 0644))
		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)
		items := []EffectiveItem{{Att: &PDFAttachment{baseAttachment: baseAttachment{}, Path: "a.pdf"}, CWD: cwd}}
		parts, cleanup, err := ToContentPartsFromEffective(context.Background(), items)
		if cleanup != nil {
			cleanup()
		}
		require.NoError(t, err)
		require.Len(t, parts, 1)
		_, isBin := parts[0].(llmadapter.BinaryPart)
		require.True(t, isBin)
	})
}
