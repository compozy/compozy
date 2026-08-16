package frontmatter

import (
	"errors"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

type testMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

func TestSplitValidDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		lineBreak string
	}{
		{name: "lf", lineBreak: "\n"},
		{name: "crlf", lineBreak: "\r\n"},
	}

	for _, tt := range tests {
		t.Run("Should split "+tt.name+" line endings", func(t *testing.T) {
			t.Parallel()

			parts, err := Split([]byte(strings.Join([]string{
				"---",
				"name: agent",
				"description: test",
				"---",
				"Body line 1",
				"Body line 2",
			}, tt.lineBreak)))
			if err != nil {
				t.Fatalf("Split() error = %v", err)
			}

			if got, want := string(parts.Metadata), "name: agent\ndescription: test\n"; got != want {
				t.Fatalf("Split() metadata = %q, want %q", got, want)
			}
			if got, want := parts.Body, "Body line 1\nBody line 2"; got != want {
				t.Fatalf("Split() body = %q, want %q", got, want)
			}
		})
	}
}

func TestSplitErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr error
	}{
		{name: "missing", content: "plain body", wantErr: ErrMissing},
		{name: "unterminated", content: "---\nname: broken", wantErr: ErrUnterminated},
		{name: "bom", content: "\ufeff---\nname: broken\n---\nbody", wantErr: ErrBOM},
	}

	for _, tt := range tests {
		t.Run("Should report "+tt.name+" frontmatter", func(t *testing.T) {
			t.Parallel()

			_, err := Split([]byte(tt.content))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Split() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeValidDocument(t *testing.T) {
	t.Parallel()

	t.Run("Should decode metadata and return the document body", func(t *testing.T) {
		t.Parallel()

		var meta testMeta
		body, err := Decode([]byte(strings.Join([]string{
			"---",
			"name: shared",
			"description: parser",
			"---",
			"Document body",
		}, "\n")), func(data []byte) error {
			return yaml.UnmarshalWithOptions(data, &meta, yaml.Strict())
		})
		if err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		if got, want := meta.Name, "shared"; got != want {
			t.Fatalf("Decode() meta.Name = %q, want %q", got, want)
		}
		if got, want := body, "Document body"; got != want {
			t.Fatalf("Decode() body = %q, want %q", got, want)
		}
	})
}

func TestDecodeReturnsDecoderError(t *testing.T) {
	t.Parallel()

	t.Run("Should return the decoder error", func(t *testing.T) {
		t.Parallel()

		var meta testMeta
		_, err := Decode([]byte(strings.Join([]string{
			"---",
			"name: [broken",
			"---",
		}, "\n")), func(data []byte) error {
			return yaml.UnmarshalWithOptions(data, &meta, yaml.Strict())
		})
		if err == nil || !strings.Contains(err.Error(), "sequence end token") {
			t.Fatalf("Decode() error = %v, want YAML syntax failure", err)
		}
	})
}

func TestDecodeRejectsNilCallback(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a nil decoder callback", func(t *testing.T) {
		t.Parallel()

		if _, err := Decode([]byte("---\nname: shared\n---\nbody"), nil); err == nil ||
			err.Error() != "frontmatter: decode callback is required" {
			t.Fatalf("Decode(nil callback) error = %v, want callback-required failure", err)
		}
	})
}

func TestUnmarshalMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should decode metadata through the shared YAML authority", func(t *testing.T) {
		t.Parallel()

		var meta testMeta
		if err := UnmarshalMetadata([]byte("name: shared\ndescription: parser\n"), &meta); err != nil {
			t.Fatalf("UnmarshalMetadata() error = %v", err)
		}
		if got, want := meta.Name, "shared"; got != want {
			t.Fatalf("UnmarshalMetadata() name = %q, want %q", got, want)
		}
	})

	t.Run("Should reject a missing target", func(t *testing.T) {
		t.Parallel()

		if err := UnmarshalMetadata([]byte("name: shared\n"), nil); err == nil {
			t.Fatal("UnmarshalMetadata(nil) error = nil, want non-nil")
		}
	})
}

func TestFormatAndRewriteStringField(t *testing.T) {
	t.Parallel()

	t.Run("Should format canonical field order and body", func(t *testing.T) {
		t.Parallel()

		content, err := Format(struct {
			Provider string `yaml:"provider,omitempty"`
			Status   string `yaml:"status"`
			File     string `yaml:"file"`
		}{Provider: "internal", Status: "valid", File: "internal/store.go"}, "# Review\n\nEvidence")
		if err != nil {
			t.Fatalf("Format() error = %v", err)
		}
		want := strings.Join([]string{
			"---",
			"provider: internal",
			"status: valid",
			"file: internal/store.go",
			"---",
			"",
			"# Review",
			"",
			"Evidence",
			"",
		}, "\n")
		if content != want {
			t.Fatalf("Format() = %q, want %q", content, want)
		}
	})

	t.Run("Should rewrite only the selected scalar token", func(t *testing.T) {
		t.Parallel()

		content := strings.Join([]string{
			"---",
			"provider:   internal # preserve spacing and comment",
			"status: 'valid' # preserve this comment",
			"file: internal/store.go",
			"---",
			"",
			"# Review",
			"",
			"Evidence",
			"",
		}, "\n")
		rewritten, err := RewriteStringField(content, "status", "resolved")
		if err != nil {
			t.Fatalf("RewriteStringField() error = %v", err)
		}
		want := strings.Replace(content, "'valid'", "resolved", 1)
		if rewritten != want {
			t.Fatalf("RewriteStringField() = %q, want %q", rewritten, want)
		}
	})
}
