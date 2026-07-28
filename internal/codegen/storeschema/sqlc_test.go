package storeschema

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModernizeSQLCGenerated(t *testing.T) {
	t.Parallel()

	t.Run("Should replace legacy empty interfaces and preserve gofmt output", func(t *testing.T) {
		t.Parallel()

		outputPath := t.TempDir()
		source := []byte(
			"package generated\n\n" +
				"type Row struct {\n" +
				" Value interface{} `json:\"value\"`\n" +
				" Label string `json:\"label\"`\n" +
				"}\n",
		)
		filePath := filepath.Join(outputPath, "query.sql.go")
		if err := os.WriteFile(filePath, source, 0o600); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}

		if err := modernizeSQLCGenerated(outputPath); err != nil {
			t.Fatalf("modernizeSQLCGenerated() error = %v", err)
		}
		got, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("os.ReadFile() error = %v", err)
		}
		if strings.Contains(string(got), "interface{}") {
			t.Fatalf("generated source = %q, want any instead of interface{}", got)
		}
		want, err := format.Source(got)
		if err != nil {
			t.Fatalf("format.Source() error = %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("generated source is not gofmt-stable:\n%s", got)
		}
	})
}
