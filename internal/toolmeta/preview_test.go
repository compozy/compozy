package toolmeta

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestRenderBuildsSafeToolPreviews(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		toolID      string
		input       json.RawMessage
		descriptor  DescriptorMetadata
		wantPreview string
		notContains []string
	}{
		{
			name:        "Should summarize compound terminal commands without pipe tails",
			toolID:      "terminal.command",
			input:       json.RawMessage(`{"command":"ls -la | head -5 && git status"}`),
			wantPreview: "ls · git",
			notContains: []string{"-la", "head -5", "status"},
		},
		{
			name:        "Should omit positional and URL credentials from terminal previews",
			toolID:      "terminal.command",
			input:       json.RawMessage(`{"command":"curl https://admin:s3cr3t@host && mysql -pHUNTER2"}`),
			wantPreview: "curl · mysql",
			notContains: []string{"admin", "s3cr3t", "HUNTER2", "host"},
		},
		{
			name:   "Should render POSIX basename safely",
			toolID: "write_file",
			input: json.RawMessage(
				`{"file_path":"/workspace/private/report.md",` +
					`"start_line":12,"end_line":20,"content":"never publish this"}`,
			),
			wantPreview: "report.md:12-20",
			notContains: []string{"never publish this", "/workspace/private"},
		},
		{
			name:   "Should render Windows drive file basename without directories",
			toolID: "write_file",
			input: json.RawMessage(
				`{"file_path":"C:\\workspace\\private\\report.md",` +
					`"start_line":7,"content":"never publish this"}`,
			),
			wantPreview: "report.md:7",
			notContains: []string{"never publish this", "C:", "workspace", "private"},
		},
		{
			name:        "Should render UNC file basename without server share or directories",
			toolID:      "read_file",
			input:       json.RawMessage(`{"path":"\\\\fileserver\\private-share\\finance\\report.md"}`),
			wantPreview: "report.md",
			notContains: []string{"fileserver", "private-share", "finance"},
		},
		{
			name:        "Should omit file preview for empty paths even when content is present",
			toolID:      "write_file",
			input:       json.RawMessage(`{"file_path":"","content":"never publish this"}`),
			wantPreview: "",
			notContains: []string{"never publish this"},
		},
		{
			name:        "Should summarize delegated tasks",
			toolID:      "delegate",
			input:       json.RawMessage(`{"tasks":[{"goal":"Inspect API"},{"goal":"Run tests"}]}`),
			wantPreview: "2 tasks: Inspect API; Run tests",
		},
		{
			name:        "Should render search query",
			toolID:      "search",
			input:       json.RawMessage(`{"query":"delivery progress"}`),
			wantPreview: "delivery progress",
		},
		{
			name:        "Should honor descriptor presentation hints",
			toolID:      "mcp__issues_lookup",
			input:       json.RawMessage(`{"needle":"BUG-123"}`),
			descriptor:  DescriptorMetadata{FriendlyVerb: "Looking up", Preview: "arg:needle"},
			wantPreview: "BUG-123",
		},
		{
			name:        "Should preserve case-sensitive descriptor argument names",
			toolID:      "mcp__issues_lookup",
			input:       json.RawMessage(`{"issueKey":"BUG-456"}`),
			descriptor:  DescriptorMetadata{FriendlyVerb: "Looking up", Preview: "arg:issueKey"},
			wantPreview: "BUG-456",
		},
		{
			name:        "Should prefer an explicit argument hint over tool id heuristics",
			toolID:      "vendor__search_issues",
			input:       json.RawMessage(`{"issueKey":"BUG-789","query":"wrong-field"}`),
			descriptor:  DescriptorMetadata{FriendlyVerb: "Looking up", Preview: "arg:issueKey"},
			wantPreview: "BUG-789",
		},
		{
			name:        "Should never render a sensitive descriptor argument",
			toolID:      "mcp__signing_key",
			input:       json.RawMessage(`{"privateKey":"raw-material-without-secret-shape"}`),
			descriptor:  DescriptorMetadata{FriendlyVerb: "Signing", Preview: "arg:privateKey"},
			wantPreview: "",
			notContains: []string{"raw-material-without-secret-shape"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := Render(tc.toolID, tc.input, RenderOptions{Descriptor: tc.descriptor, PreviewLimit: 160})
			if got.Preview != tc.wantPreview {
				t.Fatalf("Render(%q).Preview = %q, want %q", tc.toolID, got.Preview, tc.wantPreview)
			}
			for _, forbidden := range tc.notContains {
				if strings.Contains(got.Preview, forbidden) {
					t.Fatalf("Render(%q).Preview = %q contains forbidden %q", tc.toolID, got.Preview, forbidden)
				}
			}
			if tc.descriptor.FriendlyVerb != "" && got.Label != tc.descriptor.FriendlyVerb {
				t.Fatalf("Render(%q).Label = %q, want %q", tc.toolID, got.Label, tc.descriptor.FriendlyVerb)
			}
		})
	}
}

func TestRenderRedactsBeforeCappingAndFormatsUnknownFallback(t *testing.T) {
	t.Parallel()

	t.Run("Should redact the complete preview before applying its cap", func(t *testing.T) {
		t.Parallel()

		got := Render(
			"terminal.command",
			json.RawMessage(`{"command":"deploy --token=agh_claim_secret-value then-continue"}`),
			RenderOptions{PreviewLimit: 24},
		)
		if strings.Contains(got.Preview, "agh_claim_secret-value") {
			t.Fatalf("Render().Preview = %q leaked claim token", got.Preview)
		}
		if len([]rune(got.Preview)) > 24 {
			t.Fatalf("len(Render().Preview) = %d, want <= 24 runes", len([]rune(got.Preview)))
		}
	})

	t.Run("Should leave verbose previews uncapped while still redacting", func(t *testing.T) {
		t.Parallel()

		input := json.RawMessage(`{"query":"a deliberately long query with api_key=secret-material-value"}`)
		got := Render("search", input, RenderOptions{PreviewLimit: 12, Verbose: true})
		if len([]rune(got.Preview)) <= 12 {
			t.Fatalf("verbose preview = %q, want uncapped", got.Preview)
		}
		if strings.Contains(got.Preview, "secret-material-value") {
			t.Fatalf("verbose preview = %q leaked secret", got.Preview)
		}
	})

	t.Run("Should format unknown tools with the raw safe fallback", func(t *testing.T) {
		t.Parallel()

		got := Render(
			"vendor__lookup",
			json.RawMessage(`{"query":"BUG-123"}`),
			RenderOptions{PreviewLimit: 160},
		)
		if want := `⚙️ vendor__lookup: "BUG-123"`; got.String() != want {
			t.Fatalf("Render().String() = %q, want %q", got.String(), want)
		}
	})

	t.Run("Should quote fallback previews containing quotes and backslashes", func(t *testing.T) {
		t.Parallel()

		got := Render(
			"vendor__lookup",
			json.RawMessage(`{"query":"say \"hi\" \\tmp"}`),
			RenderOptions{PreviewLimit: 160},
		)
		if want := `⚙️ vendor__lookup: "say \"hi\" \\tmp"`; got.String() != want {
			t.Fatalf("Render().String() = %q, want %q", got.String(), want)
		}
	})

	t.Run("Should keep the final quoted fallback within the resolved preview cap", func(t *testing.T) {
		t.Parallel()

		got := Render(
			"vendor__lookup",
			json.RawMessage(`{"query":"1234567890\\quoted\"tail"}`),
			RenderOptions{PreviewLimit: 14},
		)
		_, preview := got.ProgressFields()
		if runes := len([]rune(preview)); runes > 14 {
			t.Fatalf("fallback preview rune count = %d, want <= 14; preview = %q", runes, preview)
		}
		if preview != "" {
			if _, err := strconv.Unquote(preview); err != nil {
				t.Fatalf("fallback preview = %q, want a valid quoted value: %v", preview, err)
			}
		}
	})

	t.Run("Should redact a secret-shaped fallback label before rendering", func(t *testing.T) {
		t.Parallel()

		got := Render(
			"agh_claim_secret-value",
			json.RawMessage(`{"query":"BUG-789"}`),
			RenderOptions{PreviewLimit: 160},
		)
		if want := `⚙️ agh_claim_[REDACTED]: "BUG-789"`; got.String() != want {
			t.Fatalf("Render().String() = %q, want %q", got.String(), want)
		}
	})
}
