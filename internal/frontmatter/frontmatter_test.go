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
		t.Run(tt.name, func(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
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
}

func TestDecodeReturnsDecoderError(t *testing.T) {
	t.Parallel()

	var meta testMeta
	_, err := Decode([]byte(strings.Join([]string{
		"---",
		"name: [broken",
		"---",
	}, "\n")), func(data []byte) error {
		return yaml.UnmarshalWithOptions(data, &meta, yaml.Strict())
	})
	if err == nil {
		t.Fatal("Decode() error = nil, want non-nil")
	}
}

func TestDecodeRejectsNilCallback(t *testing.T) {
	t.Parallel()

	if _, err := Decode([]byte("---\nname: shared\n---\nbody"), nil); err == nil {
		t.Fatal("Decode(nil callback) error = nil, want non-nil")
	}
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
