package opendesign

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

// Suite: native lint execution. Owns upstream behavior and workspace/process boundaries.
// Daemon publication and end-user authoring are verified through the runtime QA journey.
func TestLint(t *testing.T) {
	t.Run("Should preserve every original lint assertion against the shipped JavaScript", func(t *testing.T) {
		t.Parallel()
		output, err := exec.CommandContext(
			t.Context(), "bun", "test", "linter/__tests__/index.test.ts",
		).CombinedOutput()
		if err != nil {
			t.Fatalf("upstream parity: %v\n%s", err, output)
		}
		t.Logf("%s", output)
	})
	t.Run("Should lint actual workspace bytes with original findings and native feedback", func(t *testing.T) {
		t.Parallel()
		scope := lintWorkspace(t)
		name := "docs/design/feature/index.html"
		html := `<style>.cta { background: #6366f1; }</style><button>Select sessions</button>`
		writeLintHTML(t, scope, name, html)
		result, err := handleLint(t.Context(), compozysdk.ToolRequest[lintInput]{
			TrustedWorkspace: scope, Input: lintInput{Paths: []string{name}},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got struct {
			Passed    bool `json:"passed"`
			Artifacts []struct {
				Path     string                                                 `json:"path"`
				SHA256   string                                                 `json:"sha256"`
				Feedback string                                                 `json:"feedback"`
				Findings []struct{ ID, Severity, Message, Fix, Snippet string } `json:"findings"`
			} `json:"artifacts"`
		}
		if err := json.Unmarshal(result.Structured, &got); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Passed {
			t.Fatal("expected false")
		}
		if got := len(got.Artifacts); got != 1 {
			t.Fatalf("length = %d, want %d", got, 1)
		}
		artifact := got.Artifacts[0]
		if got := artifact.Path; got != name {
			t.Fatalf("got %v, want %v", got, name)
		}
		digest := sha256.Sum256([]byte(html))
		if got := artifact.SHA256; got != hex.EncodeToString(digest[:]) {
			t.Fatalf("got %v, want %v", got, hex.EncodeToString(digest[:]))
		}
		if got := len(artifact.Findings); got != 1 {
			t.Fatalf("length = %d, want %d", got, 1)
		}
		finding := artifact.Findings[0]
		if got := finding.ID; got != "ai-default-indigo" {
			t.Fatalf("got %v, want %v", got, "ai-default-indigo")
		}
		if got := finding.Severity; got != "P0" {
			t.Fatalf("got %v, want %v", got, "P0")
		}
		if got := finding.Message; got != "Found a default LLM accent color (#6366f1) — this is the most-reported AI design tell." {
			t.Fatalf(
				"got %v, want %v",
				got,
				"Found a default LLM accent color (#6366f1) — this is the most-reported AI design tell.",
			)
		}
		if got := finding.Snippet; got != "#6366f1" {
			t.Fatalf("got %v, want %v", got, "#6366f1")
		}
		if !strings.Contains(artifact.Feedback, finding.Message) {
			t.Fatalf("unexpected text: %s", artifact.Feedback)
		}
		if !strings.Contains(artifact.Feedback, finding.Fix) {
			t.Fatalf("unexpected text: %s", artifact.Feedback)
		}
		if !strings.Contains(artifact.Feedback, "Update the same workspace HTML file") {
			t.Fatalf("unexpected text: %s", artifact.Feedback)
		}
		if strings.Contains(artifact.Feedback, "Re-emit") {
			t.Fatalf("unexpected text: %s", artifact.Feedback)
		}
		bytes, err := os.ReadFile(filepath.Join(scope.Root, name))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := string(bytes); got != html {
			t.Fatalf("got %v, want %v", got, html)
		}
	})
	t.Run("Should accept intentional brand tokens without changing upstream exemptions", func(t *testing.T) {
		t.Parallel()
		input, err := json.Marshal(
			[]lintDocument{
				{
					Path: "docs/design/index.html",
					HTML: `<style>:root { --accent: #6366f1; } .cta { background: var(--accent); }</style><button>Continue</button>`,
				},
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		output, err := executeLint(t.Context(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got struct {
			Passed bool `json:"passed"`
		}
		if err := json.Unmarshal(output, &got); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.Passed {
			t.Fatal("expected true: got.Passed")
		}
	})
	t.Run("Should reject malformed child input without evaluating HTML as code", func(t *testing.T) {
		t.Parallel()
		_, err := executeLint(t.Context(), []byte("{"))
		if err == nil || !strings.Contains(err.Error(), "linter failed") {
			t.Fatalf("error = %v, want containing %q", err, "linter failed")
		}
	})
	t.Run("Should preserve cancellation and bound output", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := executeLint(ctx, []byte("[]"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
		_, err = handleLint(
			ctx,
			compozysdk.ToolRequest[lintInput]{
				TrustedWorkspace: lintWorkspace(t),
				Input:            lintInput{Paths: []string{"docs/design/index.html"}},
			},
		)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
		buffer := limitedBuffer{remaining: 3}
		_, err = buffer.Write([]byte("four"))
		if err == nil {
			t.Fatal("expected error")
		}
		if len(buffer.Bytes()) != 0 {
			t.Fatal("expected empty output")
		}
	})
	t.Run("Should explain the missing Node dependency", func(t *testing.T) {
		// not parallel: t.Setenv isolates process PATH for this dependency failure.
		t.Setenv("PATH", t.TempDir())
		_, err := executeLint(t.Context(), []byte("[]"))
		if err == nil || !strings.Contains(err.Error(), "Node.js is required") {
			t.Fatalf("error = %v, want containing %q", err, "Node.js is required")
		}
	})
	t.Run("Should ignore Node preload settings", func(t *testing.T) {
		// not parallel: t.Setenv isolates Node preload variables for this case.
		t.Setenv("NODE_OPTIONS", "--require=/nonexistent/open-design-preload.cjs")
		t.Setenv("NODE_PATH", t.TempDir())
		result, err := executeLint(t.Context(), []byte("[]"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got struct {
			Passed    bool  `json:"passed"`
			Artifacts []any `json:"artifacts"`
		}
		if err := json.Unmarshal(result, &got); err != nil || !got.Passed || len(got.Artifacts) != 0 {
			t.Fatalf("result = %s, error = %v", result, err)
		}
	})
}

func TestLintWorkspaceBoundary(t *testing.T) {
	t.Parallel()
	t.Run("Should reject untrusted and out-of-scope paths", func(t *testing.T) {
		t.Parallel()
		scope := lintWorkspace(t)
		writeLintHTML(t, scope, "docs/design/index.html", "<p>Sessions</p>")
		for _, name := range []string{"../index.html", "/docs/design/index.html", "docs/design/../index.html", "docs/design//index.html", "docs/design/./index.html", `docs\design\index.html`, "docs/design/index.js", "docs/design/"} {
			_, err := readArtifacts(t.Context(), scope, []string{name})
			if err == nil {
				t.Fatal("expected error")
			}
		}
		for _, invalid := range []*compozysdk.ExtensionToolWorkspaceScope{nil, {Root: scope.Root}, {ID: "workspace", Root: "relative"}} {
			_, err := readArtifacts(t.Context(), invalid, []string{"docs/design/index.html"})
			if err == nil || !strings.Contains(err.Error(), "trusted workspace") {
				t.Fatalf("error = %v, want containing %q", err, "trusted workspace")
			}
		}
		_, err := readArtifacts(t.Context(), scope, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		_, err = readArtifacts(t.Context(), scope, make([]string, maxArtifacts+1))
		if err == nil {
			t.Fatal("expected error")
		}
		_, err = readArtifacts(t.Context(), scope, []string{"docs/design/index.html", "docs/design/index.html"})
		if err == nil || !strings.Contains(err.Error(), "duplicate artifact") {
			t.Fatalf("error = %v, want containing %q", err, "duplicate artifact")
		}
	})
	t.Run("Should reject symlinks at file and directory boundaries", func(t *testing.T) {
		t.Parallel()
		scope := lintWorkspace(t)
		outside := lintWorkspace(t)
		writeLintHTML(t, outside, "docs/design/private.html", "<p>Private</p>")
		if err := os.Symlink(
			filepath.Join(outside.Root, "docs", "design", "private.html"),
			filepath.Join(scope.Root, "docs", "design", "link.html"),
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err := readArtifacts(t.Context(), scope, []string{"docs/design/link.html"})
		if err == nil {
			t.Fatal("expected error")
		}
		if err := os.Symlink(
			filepath.Join(outside.Root, "docs", "design"),
			filepath.Join(scope.Root, "docs", "design", "linked"),
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = readArtifacts(t.Context(), scope, []string{"docs/design/linked/private.html"})
		if err == nil {
			t.Fatal("expected error")
		}
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := os.Symlink(
			filepath.Join(outside.Root, "docs", "design"),
			filepath.Join(root, "docs", "design"),
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = readArtifacts(
			t.Context(),
			&compozysdk.ExtensionToolWorkspaceScope{ID: "workspace", Root: root},
			[]string{"docs/design/private.html"},
		)
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("Should reject unreadable empty invalid and oversized artifacts", func(t *testing.T) {
		t.Parallel()
		scope := lintWorkspace(t)
		for _, html := range []string{" \n", string([]byte{0xff}), strings.Repeat("a", maxArtifactBytes+1)} {
			writeLintHTML(t, scope, "docs/design/index.html", html)
			_, err := readArtifacts(t.Context(), scope, []string{"docs/design/index.html"})
			if err == nil {
				t.Fatal("expected error")
			}
		}
		_, err := readArtifacts(t.Context(), scope, []string{"docs/design/missing.html"})
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing artifact: %v, want os.ErrNotExist", err)
		}
		if err := os.Mkdir(filepath.Join(scope.Root, "docs", "design", "directory.html"), 0o755); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = readArtifacts(t.Context(), scope, []string{"docs/design/directory.html"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	// Invariant: the aggregate cap applies even when each individual file is valid.
	// Owner: native lint input boundary; reuse the workspace-boundary suite.
	t.Run("Should enforce the combined input byte limit", func(t *testing.T) {
		t.Parallel()
		scope := lintWorkspace(t)
		paths := make([]string, maxInputBytes/maxArtifactBytes+1)
		for i := range paths {
			paths[i] = fmt.Sprintf("docs/design/batch-%d.html", i)
			writeLintHTML(t, scope, paths[i], strings.Repeat("a", maxArtifactBytes))
		}
		if _, err := readArtifacts(t.Context(), scope, paths[:len(paths)-1]); err != nil {
			t.Fatalf("input at the limit: %v", err)
		}
		if _, err := readArtifacts(
			t.Context(),
			scope,
			paths,
		); err == nil ||
			!strings.Contains(err.Error(), "combined HTML exceeds") {
			t.Fatalf("input above the limit: %v", err)
		}
	})
}

func lintWorkspace(t *testing.T) *compozysdk.ExtensionToolWorkspaceScope {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "design"), 0o755); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return &compozysdk.ExtensionToolWorkspaceScope{ID: "workspace", Root: root}
}

func writeLintHTML(t *testing.T, scope *compozysdk.ExtensionToolWorkspaceScope, name, html string) {
	t.Helper()
	filename := filepath.Join(scope.Root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := os.WriteFile(filename, []byte(html), 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
