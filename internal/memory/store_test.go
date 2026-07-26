package memory

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
	"github.com/goccy/go-yaml"
)

type testMemoryMeta struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description,omitempty"`
	Type        memcontract.Type `yaml:"type"`
	AgentName   string           `yaml:"agent,omitempty"`
}

func TestStoreWriteReadRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		scope        memcontract.Scope
		filename     string
		meta         testMemoryMeta
		body         string
		wantLocation func(*testStoreEnv) string
	}{
		{
			name:     "global scope",
			scope:    memcontract.ScopeGlobal,
			filename: "user_preferences.md",
			meta: testMemoryMeta{
				Name:        "User Preferences",
				Description: "Preferred working style",
				Type:        memcontract.TypeUser,
			},
			body: "Prefers explicit error handling.\n",
			wantLocation: func(env *testStoreEnv) string {
				return filepath.Join(env.store.globalDir, "user_preferences.md")
			},
		},
		{
			name:     "workspace scope",
			scope:    memcontract.ScopeWorkspace,
			filename: "project_auth.md",
			meta: testMemoryMeta{
				Name:        "Auth Rewrite",
				Description: "JWT rollout plan",
				Type:        memcontract.TypeProject,
			},
			body: "Workspace uses JWT-based auth.\n",
			wantLocation: func(env *testStoreEnv) string {
				return filepath.Join(env.store.workspaceDir, "project_auth.md")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := newTestStoreEnv(t)
			payload := mustMemoryContent(t, tt.meta, tt.body)

			if err := env.store.Write(tt.scope, tt.filename, payload); err != nil {
				t.Fatalf("Store.Write() error = %v", err)
			}

			got, err := env.store.Read(tt.scope, tt.filename)
			if err != nil {
				t.Fatalf("Store.Read() error = %v", err)
			}
			if !bytes.Equal(got, payload) {
				t.Fatalf("Store.Read() = %q, want %q", string(got), string(payload))
			}

			if _, err := os.Stat(tt.wantLocation(env)); err != nil {
				t.Fatalf("os.Stat(written path) error = %v", err)
			}
		})
	}
}

func TestStoreWriteRejectsInvalidFrontmatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "missing name",
			content: `---
type: user
---
Body
`,
			wantErr: "memory name is required",
		},
		{
			name: "missing type",
			content: `---
name: Missing Type
---
Body
`,
			wantErr: "memory type is required",
		},
		{
			name: "unknown type",
			content: `---
name: Unknown Type
type: archive
---
Body
`,
			wantErr: `unsupported memory type "archive"`,
		},
		{
			name:    "missing frontmatter",
			content: "Body only\n",
			wantErr: "missing YAML frontmatter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := newTestStoreEnv(t)
			err := env.store.Write(memcontract.ScopeGlobal, "invalid.md", []byte(tt.content))
			if err == nil {
				t.Fatal("Store.Write() error = nil, want non-nil")
			}
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("Store.Write() error = %v, want ErrValidation", err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Store.Write() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestStoreWriteRejectsInvalidFilename(t *testing.T) {
	t.Parallel()
	payload := mustMemoryContent(t, testMemoryMeta{
		Name:        "Valid",
		Description: "Validation test",
		Type:        memcontract.TypeUser,
	}, "Body\n")

	tests := []struct {
		name     string
		filename string
		wantErr  string
	}{
		{name: "path separator", filename: "nested/file.md", wantErr: "must not include path separators"},
		{name: "backslash separator", filename: `nested\file.md`, wantErr: "must not include path separators"},
		{name: "dot", filename: ".", wantErr: `filename "." is invalid`},
		{name: "dotdot", filename: "..", wantErr: `filename ".." is invalid`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := newTestStoreEnv(t)
			err := env.store.Write(memcontract.ScopeGlobal, tt.filename, payload)
			if err == nil {
				t.Fatal("Store.Write() error = nil, want non-nil")
			}
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("Store.Write() error = %v, want ErrValidation", err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Store.Write() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestStoreReadMissingFile(t *testing.T) {
	t.Parallel()

	env := newTestStoreEnv(t)

	_, err := env.store.Read(memcontract.ScopeGlobal, "missing.md")
	if err == nil {
		t.Fatal("Store.Read() error = nil, want non-nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Store.Read() error = %v, want os.ErrNotExist", err)
	}
}

func TestStoreDeleteRemovesFileAndIndexEntry(t *testing.T) {
	t.Parallel()

	env := newTestStoreEnv(t)
	payload := mustMemoryContent(t, testMemoryMeta{
		Name:        "User Preferences",
		Description: "Preferred tools",
		Type:        memcontract.TypeUser,
	}, "Prefers rg over grep.\n")

	if err := env.store.Write(memcontract.ScopeGlobal, "user_preferences.md", payload); err != nil {
		t.Fatalf("Store.Write() error = %v", err)
	}

	indexContent := strings.Join([]string{
		"- [User Preferences](user_preferences.md) - Preferred tools",
		"- [Other](other.md) - Another note",
		"",
	}, "\n")
	if err := os.WriteFile(
		filepath.Join(env.store.globalDir, indexFilename),
		[]byte(indexContent),
		filePerm,
	); err != nil {
		t.Fatalf("write index file: %v", err)
	}

	if err := env.store.Delete(memcontract.ScopeGlobal, "user_preferences.md"); err != nil {
		t.Fatalf("Store.Delete() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(env.store.globalDir, "user_preferences.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(deleted file) error = %v, want os.ErrNotExist", err)
	}

	indexBytes, err := os.ReadFile(filepath.Join(env.store.globalDir, indexFilename))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.ReadFile(index) error = %v", err)
		}
		return
	}
	if strings.Contains(string(indexBytes), "(user_preferences.md)") {
		t.Fatalf("index content still references deleted file: %q", string(indexBytes))
	}
}

func TestStoreDeletePreservesLinesThatOnlyMentionFilenameInDescription(t *testing.T) {
	t.Parallel()

	env := newTestStoreEnv(t)
	payload := mustMemoryContent(t, testMemoryMeta{
		Name:        "User Preferences",
		Description: "Preferred tools",
		Type:        memcontract.TypeUser,
	}, "Prefers rg over grep.\n")

	if err := env.store.Write(memcontract.ScopeGlobal, "user_preferences.md", payload); err != nil {
		t.Fatalf("Store.Write() error = %v", err)
	}

	indexContent := strings.Join([]string{
		"- [User Preferences](user_preferences.md) - Preferred tools",
		"- [Related Notes](other.md) - Mirrors notes from (user_preferences.md)",
		"",
	}, "\n")
	if err := os.WriteFile(
		filepath.Join(env.store.globalDir, indexFilename),
		[]byte(indexContent),
		filePerm,
	); err != nil {
		t.Fatalf("write index file: %v", err)
	}

	if err := env.store.Delete(memcontract.ScopeGlobal, "user_preferences.md"); err != nil {
		t.Fatalf("Store.Delete() error = %v", err)
	}

	indexBytes, err := os.ReadFile(filepath.Join(env.store.globalDir, indexFilename))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.ReadFile(index) error = %v", err)
		}
		return
	}
	got := string(indexBytes)
	if strings.Contains(got, "- [User Preferences](user_preferences.md)") {
		t.Fatalf("index content still references deleted file: %q", got)
	}
}

func TestStoreDeleteRemovesIndexEntryForFilenameWithParentheses(t *testing.T) {
	t.Parallel()

	env := newTestStoreEnv(t)
	filename := "user(preferences).md"
	payload := mustMemoryContent(t, testMemoryMeta{
		Name:        "User Preferences",
		Description: "Preferred tools",
		Type:        memcontract.TypeUser,
	}, "Prefers rg over grep.\n")

	if err := env.store.Write(memcontract.ScopeGlobal, filename, payload); err != nil {
		t.Fatalf("Store.Write() error = %v", err)
	}

	indexContent := strings.Join([]string{
		"- [User Preferences](user(preferences).md) - Preferred tools",
		"- [Other](other.md) - Another note",
		"",
	}, "\n")
	if err := os.WriteFile(
		filepath.Join(env.store.globalDir, indexFilename),
		[]byte(indexContent),
		filePerm,
	); err != nil {
		t.Fatalf("write index file: %v", err)
	}

	if err := env.store.Delete(memcontract.ScopeGlobal, filename); err != nil {
		t.Fatalf("Store.Delete() error = %v", err)
	}

	indexBytes, err := os.ReadFile(filepath.Join(env.store.globalDir, indexFilename))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.ReadFile(index) error = %v", err)
		}
		return
	}
	got := string(indexBytes)
	if strings.Contains(got, "- [User Preferences](user(preferences).md)") {
		t.Fatalf("index content still references deleted file: %q", got)
	}
}

func TestStoreDeleteMissingFile(t *testing.T) {
	t.Parallel()

	env := newTestStoreEnv(t)

	err := env.store.Delete(memcontract.ScopeWorkspace, "missing.md")
	if err == nil {
		t.Fatal("Store.Delete() error = nil, want non-nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Store.Delete() error = %v, want os.ErrNotExist", err)
	}
}

func TestStoreScanReturnsNewestFirst(t *testing.T) {
	t.Parallel()

	env := newTestStoreEnv(t)
	base := time.Now().Add(-3 * time.Hour)

	files := []struct {
		filename string
		name     string
		modTime  time.Time
		agent    string
	}{
		{filename: "older.md", name: "Older", modTime: base},
		{filename: "newest.md", name: "Newest", modTime: base.Add(2 * time.Hour)},
		{filename: "middle.md", name: "Middle", modTime: base.Add(1 * time.Hour)},
	}

	for _, file := range files {
		payload := mustMemoryContent(t, testMemoryMeta{
			Name:        file.name,
			Description: file.name + " description",
			Type:        memcontract.TypeProject,
			AgentName:   file.agent,
		}, file.name+" body\n")
		if err := env.store.Write(memcontract.ScopeWorkspace, file.filename, payload); err != nil {
			t.Fatalf("Store.Write(%q) error = %v", file.filename, err)
		}

		path, err := env.store.pathFor(memcontract.ScopeWorkspace, file.filename)
		if err != nil {
			t.Fatalf("pathFor(%q) error = %v", file.filename, err)
		}
		if err := os.Chtimes(path, file.modTime, file.modTime); err != nil {
			t.Fatalf("os.Chtimes(%q) error = %v", path, err)
		}
	}

	headers, err := env.store.Scan(memcontract.ScopeWorkspace)
	if err != nil {
		t.Fatalf("Store.Scan() error = %v", err)
	}

	if got, want := len(headers), 3; got != want {
		t.Fatalf("len(headers) = %d, want %d", got, want)
	}

	wantOrder := []string{"newest.md", "middle.md", "older.md"}
	for idx, want := range wantOrder {
		if headers[idx].Filename != want {
			t.Fatalf("headers[%d].Filename = %q, want %q", idx, headers[idx].Filename, want)
		}
		if headers[idx].FilePath == "" {
			t.Fatalf("headers[%d].FilePath = empty, want populated path", idx)
		}
	}

	if headers[0].AgentName != "" {
		t.Fatalf("headers[0].AgentName = %q, want empty string", headers[0].AgentName)
	}
	if headers[1].AgentName != "" {
		t.Fatalf("headers[1].AgentName = %q, want empty string", headers[1].AgentName)
	}
}

func TestStoreScanSkipsAtomicTempFiles(t *testing.T) {
	t.Parallel()

	t.Run("Should ignore temp files left visible during concurrent atomic writes", func(t *testing.T) {
		t.Parallel()

		env := newTestStoreEnv(t)
		valid := mustMemoryContent(t, testMemoryMeta{
			Name:        "Project",
			Description: "Visible project memory",
			Type:        memcontract.TypeProject,
		}, "stable body\n")
		if err := env.store.Write(memcontract.ScopeWorkspace, "project.md", valid); err != nil {
			t.Fatalf("Store.Write(project.md) error = %v", err)
		}

		tempContent := mustMemoryContent(t, testMemoryMeta{
			Name:        "Temp",
			Description: "Atomic temp file should not be indexed",
			Type:        memcontract.TypeProject,
		}, "temp body\n")
		tempPath := filepath.Join(env.store.workspaceDir, "project.md.tmp-123456")
		if err := os.WriteFile(tempPath, tempContent, filePerm); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", tempPath, err)
		}

		headers, err := env.store.Scan(memcontract.ScopeWorkspace)
		if err != nil {
			t.Fatalf("Store.Scan() error = %v", err)
		}
		if got, want := len(headers), 1; got != want {
			t.Fatalf("len(headers) = %d, want %d", got, want)
		}
		if got, want := headers[0].Filename, "project.md"; got != want {
			t.Fatalf("headers[0].Filename = %q, want %q", got, want)
		}

		targets, err := env.store.ListTargets(context.Background(), memcontract.Candidate{
			Scope:   memcontract.ScopeWorkspace,
			Content: "new memory",
		})
		if err != nil {
			t.Fatalf("Store.ListTargets() error = %v", err)
		}
		if got, want := len(targets), 1; got != want {
			t.Fatalf("len(targets) = %d, want %d", got, want)
		}
		if strings.Contains(targets[0].ID, ".tmp-") || strings.Contains(targets[0].TargetFilename, ".tmp-") {
			t.Fatalf("ListTargets() leaked temp target = %#v", targets[0])
		}
	})
}

func TestStoreScanCapsAtTwoHundredFiles(t *testing.T) {
	t.Run("Should cap prompt scans while counting every valid source header", func(t *testing.T) {
		t.Parallel()

		env := newTestStoreEnv(t)
		base := time.Now().Add(-205 * time.Minute)

		for idx := range 205 {
			filename := fmt.Sprintf("%03d.md", idx)
			payload := mustMemoryContent(t, testMemoryMeta{
				Name:        fmt.Sprintf("Memory %03d", idx),
				Description: "Cap test",
				Type:        memcontract.TypeReference,
			}, "Reference entry\n")
			if err := env.store.Write(memcontract.ScopeWorkspace, filename, payload); err != nil {
				t.Fatalf("Store.Write(%q) error = %v", filename, err)
			}

			path, err := env.store.pathFor(memcontract.ScopeWorkspace, filename)
			if err != nil {
				t.Fatalf("pathFor(%q) error = %v", filename, err)
			}
			modTime := base.Add(time.Duration(idx) * time.Minute)
			if err := os.Chtimes(path, modTime, modTime); err != nil {
				t.Fatalf("os.Chtimes(%q) error = %v", path, err)
			}
		}

		headers, err := env.store.Scan(memcontract.ScopeWorkspace)
		if err != nil {
			t.Fatalf("Store.Scan() error = %v", err)
		}

		if got, want := len(headers), 200; got != want {
			t.Fatalf("len(headers) = %d, want %d", got, want)
		}
		if headers[0].Filename != "204.md" {
			t.Fatalf("headers[0].Filename = %q, want %q", headers[0].Filename, "204.md")
		}
		if headers[len(headers)-1].Filename != "005.md" {
			t.Fatalf("headers[last].Filename = %q, want %q", headers[len(headers)-1].Filename, "005.md")
		}

		count, err := env.store.SourceHeaderCount(memcontract.ScopeWorkspace)
		if err != nil {
			t.Fatalf("Store.SourceHeaderCount() error = %v", err)
		}
		if got, want := count, 205; got != want {
			t.Fatalf("Store.SourceHeaderCount() = %d, want %d", got, want)
		}
	})
}

func TestStoreScanCapsAtTwoHundredFilesAfterSkippingMalformedNewestEntries(t *testing.T) {
	t.Parallel()

	env := newTestStoreEnv(t)
	base := time.Now().Add(-205 * time.Minute)

	for idx := range 205 {
		filename := fmt.Sprintf("%03d.md", idx)
		payload := mustMemoryContent(t, testMemoryMeta{
			Name:        fmt.Sprintf("Memory %03d", idx),
			Description: "Cap test",
			Type:        memcontract.TypeReference,
		}, "Reference entry\n")
		if err := env.store.Write(memcontract.ScopeWorkspace, filename, payload); err != nil {
			t.Fatalf("Store.Write(%q) error = %v", filename, err)
		}

		path, err := env.store.pathFor(memcontract.ScopeWorkspace, filename)
		if err != nil {
			t.Fatalf("pathFor(%q) error = %v", filename, err)
		}
		modTime := base.Add(time.Duration(idx) * time.Minute)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("os.Chtimes(%q) error = %v", path, err)
		}
	}

	for idx := range 3 {
		filename := fmt.Sprintf("broken-%d.md", idx)
		path, err := env.store.pathFor(memcontract.ScopeWorkspace, filename)
		if err != nil {
			t.Fatalf("pathFor(%q) error = %v", filename, err)
		}
		if err := os.WriteFile(path, []byte("not frontmatter\n"), filePerm); err != nil {
			t.Fatalf("write malformed file %q: %v", filename, err)
		}
		modTime := base.Add(time.Duration(205+idx) * time.Minute)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("os.Chtimes(%q) error = %v", path, err)
		}
	}

	headers, err := env.store.Scan(memcontract.ScopeWorkspace)
	if err != nil {
		t.Fatalf("Store.Scan() error = %v", err)
	}

	if got, want := len(headers), 200; got != want {
		t.Fatalf("len(headers) = %d, want %d", got, want)
	}
	if headers[0].Filename != "204.md" {
		t.Fatalf("headers[0].Filename = %q, want %q", headers[0].Filename, "204.md")
	}
	if headers[len(headers)-1].Filename != "005.md" {
		t.Fatalf("headers[last].Filename = %q, want %q", headers[len(headers)-1].Filename, "005.md")
	}
}

func TestStoreScanSkipsMalformedFilesAndLogsWarning(t *testing.T) {
	t.Parallel()

	env := newTestStoreEnv(t)
	var logs bytes.Buffer
	env.store.logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))

	if err := env.store.Write(memcontract.ScopeGlobal, "valid.md", mustMemoryContent(t, testMemoryMeta{
		Name:        "Valid",
		Description: "Valid memory",
		Type:        memcontract.TypeFeedback,
	}, "Valid body\n")); err != nil {
		t.Fatalf("Store.Write(valid) error = %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(env.store.globalDir, "broken.md"),
		[]byte("not frontmatter\n"),
		filePerm,
	); err != nil {
		t.Fatalf("write malformed file: %v", err)
	}

	headers, err := env.store.Scan(memcontract.ScopeGlobal)
	if err != nil {
		t.Fatalf("Store.Scan() error = %v", err)
	}
	if got, want := len(headers), 1; got != want {
		t.Fatalf("len(headers) = %d, want %d", got, want)
	}
	if !strings.Contains(logs.String(), "skip malformed memory file") {
		t.Fatalf("scan logs = %q, want malformed warning", logs.String())
	}
}

func TestStoreLoadIndex(t *testing.T) {
	t.Parallel()

	t.Run("Should returns full content", func(t *testing.T) {
		t.Parallel()

		env := newTestStoreEnv(t)
		want := strings.Join([]string{
			"- [Auth Rewrite](project_auth.md) - JWT rollout",
			"- [Reference](reference_api.md) - API docs",
			"",
		}, "\n")
		if err := os.WriteFile(
			filepath.Join(env.store.workspaceDir, indexFilename),
			[]byte(want),
			filePerm,
		); err != nil {
			t.Fatalf("write index: %v", err)
		}
		writeIndexFixtures(t, env.store.workspaceDir, want)

		got, truncated, err := env.store.LoadIndex(memcontract.ScopeWorkspace)
		if err != nil {
			t.Fatalf("Store.LoadIndex() error = %v", err)
		}
		if truncated {
			t.Fatal("Store.LoadIndex() truncated = true, want false")
		}
		if got != want {
			t.Fatalf("Store.LoadIndex() = %q, want %q", got, want)
		}
	})

	t.Run("Should truncates by line count", func(t *testing.T) {
		t.Parallel()

		env := newTestStoreEnv(t)

		lines := make([]string, 0, 205)
		for idx := range 205 {
			lines = append(lines, fmt.Sprintf("- [Line %03d](line-%03d.md) - Description", idx, idx))
		}
		index := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(env.store.globalDir, indexFilename), []byte(index), filePerm); err != nil {
			t.Fatalf("write index: %v", err)
		}
		writeIndexFixtures(t, env.store.globalDir, index)

		got, truncated, err := env.store.LoadIndex(memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Store.LoadIndex() error = %v", err)
		}
		if !truncated {
			t.Fatal("Store.LoadIndex() truncated = false, want true")
		}

		gotLines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
		if gotCount, wantCount := len(gotLines), 200; gotCount != wantCount {
			t.Fatalf("len(gotLines) = %d, want %d", gotCount, wantCount)
		}
		if gotLines[199] != "- [Line 199](line-199.md) - Description" {
			t.Fatalf("last retained line = %q, want line 199", gotLines[199])
		}
	})

	t.Run("Should truncates by byte count and respects utf8", func(t *testing.T) {
		t.Parallel()

		env := newTestStoreEnv(t)

		line := "- [Oversized](oversized.md) - " + strings.Repeat("é", defaultIndexBytes) + "\n"
		writeIndexFixtures(t, env.store.globalDir, line)
		if err := os.WriteFile(filepath.Join(env.store.globalDir, indexFilename), []byte(line), filePerm); err != nil {
			t.Fatalf("write index: %v", err)
		}

		got, truncated, err := env.store.LoadIndex(memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Store.LoadIndex() error = %v", err)
		}
		if !truncated {
			t.Fatal("Store.LoadIndex() truncated = false, want true")
		}
		if len(got) > env.store.maxIndexBytes {
			t.Fatalf("len(got) = %d, want <= %d", len(got), env.store.maxIndexBytes)
		}
		if !utf8.ValidString(got) {
			t.Fatalf("Store.LoadIndex() returned invalid UTF-8: %q", got)
		}
	})

	t.Run("Should missing index returns empty content", func(t *testing.T) {
		t.Parallel()

		env := newTestStoreEnv(t)

		got, truncated, err := env.store.LoadIndex(memcontract.ScopeWorkspace)
		if err != nil {
			t.Fatalf("Store.LoadIndex() error = %v", err)
		}
		if truncated {
			t.Fatal("Store.LoadIndex() truncated = true, want false")
		}
		if got != "" {
			t.Fatalf("Store.LoadIndex() = %q, want empty", got)
		}
	})
}

func TestStoreLoadPromptIndexViaBackendAlias(t *testing.T) {
	t.Run("Should expose LoadIndex via the backend alias", func(t *testing.T) {
		t.Parallel()

		env := newTestStoreEnv(t)
		if err := env.store.Write(memcontract.ScopeGlobal, "prefs.md", mustMemoryContent(t, testMemoryMeta{
			Name:        "Prefs",
			Description: "Saved preference",
			Type:        memcontract.TypeUser,
		}, "body\n")); err != nil {
			t.Fatalf("Store.Write() error = %v", err)
		}

		var backend memcontract.Backend = env.store
		got, truncated, err := backend.LoadPromptIndex(memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("memcontract.Backend.LoadPromptIndex() error = %v", err)
		}
		if truncated {
			t.Fatal("memcontract.Backend.LoadPromptIndex() truncated = true, want false")
		}
		if !strings.Contains(got, "- [Prefs](prefs.md) - Saved preference") {
			t.Fatalf("memcontract.Backend.LoadPromptIndex() = %q, want rendered index entry", got)
		}
	})
}

func TestStoreLoadIndexSynthesizesWhenIndexIsMissingOrStale(t *testing.T) {
	t.Parallel()

	t.Run("Should missing index synthesizes from files", func(t *testing.T) {
		t.Parallel()

		env := newTestStoreEnv(t)
		if err := env.store.Write(memcontract.ScopeGlobal, "prefs.md", mustMemoryContent(t, testMemoryMeta{
			Name:        "Prefs",
			Description: "User preferences",
			Type:        memcontract.TypeUser,
		}, "body\n")); err != nil {
			t.Fatalf("Store.Write() error = %v", err)
		}
		if err := os.Remove(filepath.Join(env.store.globalDir, indexFilename)); err != nil {
			t.Fatalf("remove index: %v", err)
		}

		got, truncated, err := env.store.LoadIndex(memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Store.LoadIndex() error = %v", err)
		}
		if truncated {
			t.Fatal("Store.LoadIndex() truncated = true, want false")
		}
		if !strings.Contains(got, "- [Prefs](prefs.md) - User preferences") {
			t.Fatalf("Store.LoadIndex() = %q, want synthesized entry", got)
		}
	})

	t.Run("Should stale index synthesizes and ignores missing targets", func(t *testing.T) {
		t.Parallel()

		env := newTestStoreEnv(t)
		if err := env.store.Write(memcontract.ScopeWorkspace, "project.md", mustMemoryContent(t, testMemoryMeta{
			Name:        "Project",
			Description: "Current plan",
			Type:        memcontract.TypeProject,
		}, "body\n")); err != nil {
			t.Fatalf("Store.Write() error = %v", err)
		}
		stale := strings.Join([]string{
			"- [Old](missing.md) - stale",
			"- [Project](project.md) - Current plan",
			"",
		}, "\n")
		if err := os.WriteFile(
			filepath.Join(env.store.workspaceDir, indexFilename),
			[]byte(stale),
			filePerm,
		); err != nil {
			t.Fatalf("write stale index: %v", err)
		}

		got, truncated, err := env.store.LoadIndex(memcontract.ScopeWorkspace)
		if err != nil {
			t.Fatalf("Store.LoadIndex() error = %v", err)
		}
		if truncated {
			t.Fatal("Store.LoadIndex() truncated = true, want false")
		}
		if strings.Contains(got, "missing.md") || !strings.Contains(got, "project.md") {
			t.Fatalf("Store.LoadIndex() = %q, want synthesized workspace-only entry", got)
		}
	})

	t.Run("Should stale index synthesizes when metadata changes", func(t *testing.T) {
		t.Parallel()

		env := newTestStoreEnv(t)
		if err := env.store.Write(memcontract.ScopeGlobal, "prefs.md", mustMemoryContent(t, testMemoryMeta{
			Name:        "Prefs",
			Description: "Fresh description",
			Type:        memcontract.TypeUser,
		}, "body\n")); err != nil {
			t.Fatalf("Store.Write() error = %v", err)
		}

		stale := "- [Prefs](prefs.md) - stale description" + "\n" + ""
		if err := os.WriteFile(filepath.Join(env.store.globalDir, indexFilename), []byte(stale), filePerm); err != nil {
			t.Fatalf("write stale index: %v", err)
		}

		got, truncated, err := env.store.LoadIndex(memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Store.LoadIndex() error = %v", err)
		}
		if truncated {
			t.Fatal("Store.LoadIndex() truncated = true, want false")
		}
		if strings.Contains(got, "stale description") {
			t.Fatalf("Store.LoadIndex() = %q, want stale metadata rejected", got)
		}
		if !strings.Contains(got, "Fresh description") {
			t.Fatalf("Store.LoadIndex() = %q, want fresh metadata rendered", got)
		}
	})
}

func TestStoreSearchAndReindex(t *testing.T) {
	t.Run("Should reject tokenless queries before warming the catalog", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		catalogPath := filepath.Join(baseDir, "agh.db")
		store := NewStore(
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(catalogPath),
		).ForWorkspace(workspaceRoot)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}

		_, err := store.Search(
			context.Background(),
			"!!!",
			memcontract.SearchOptions{Workspace: workspaceRoot, Limit: maxSearchLimit + 25},
		)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Store.Search() error = %v, want ErrValidation", err)
		}
		if _, statErr := os.Stat(catalogPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(%q) error = %v, want catalog database to remain absent", catalogPath, statErr)
		}
	})

	t.Run("Should search and reindex visible scopes on cold start", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		catalogPath := filepath.Join(baseDir, "agh.db")
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(catalogPath),
		).ForWorkspace(workspaceRoot)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}

		if err := store.Write(memcontract.ScopeGlobal, "prefs.md", mustMemoryContent(t, testMemoryMeta{
			Name:        "Code Style",
			Description: "Keep prompts concise",
			Type:        memcontract.TypeUser,
		}, "User prefers concise answers and explicit tradeoffs.\n")); err != nil {
			t.Fatalf("Store.Write(global) error = %v", err)
		}
		if err := store.Write(memcontract.ScopeWorkspace, "auth.md", mustMemoryContent(t, testMemoryMeta{
			Name:        "Auth Rewrite",
			Description: "Workspace auth migration",
			Type:        memcontract.TypeProject,
		}, "The workspace is migrating auth from JWT to sessions.\n")); err != nil {
			t.Fatalf("Store.Write(workspace) error = %v", err)
		}

		ctx := context.Background()
		results, err := store.Search(
			ctx,
			"auth sessions concise",
			memcontract.SearchOptions{Workspace: workspaceRoot, Limit: 5},
		)
		if err != nil {
			t.Fatalf("Store.Search() error = %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("len(results) = %d, want 2; results=%#v", len(results), results)
		}
		if results[0].Scope != memcontract.ScopeWorkspace {
			t.Fatalf("results[0].Scope = %q, want workspace", results[0].Scope)
		}

		reindex, err := store.Reindex(ctx, memcontract.ReindexOptions{Workspace: workspaceRoot})
		if err != nil {
			t.Fatalf("Store.Reindex() error = %v", err)
		}
		if reindex.IndexedFiles != 2 {
			t.Fatalf("Reindex.IndexedFiles = %d, want 2", reindex.IndexedFiles)
		}

		stats, err := store.HealthStats(ctx, []string{workspaceRoot})
		if err != nil {
			t.Fatalf("Store.HealthStats() error = %v", err)
		}
		if stats.IndexedFiles != 2 || stats.OrphanedFiles != 0 || stats.LastReindex == nil {
			t.Fatalf("memcontract.HealthStats() = %#v, want indexed=2 orphaned=0 lastReindex set", stats)
		}
	})

	t.Run("Should scope health operation stats to visible workspaces", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		globalDir := filepath.Join(baseDir, "global")
		catalogPath := filepath.Join(baseDir, "agh.db")
		workspaceA := filepath.Join(baseDir, "workspace-a")
		workspaceB := filepath.Join(baseDir, "workspace-b")
		storeA := newOpenTestStore(t, globalDir, WithCatalogDatabasePath(catalogPath)).ForWorkspace(workspaceA)
		storeB := newOpenTestStore(t, globalDir, WithCatalogDatabasePath(catalogPath)).ForWorkspace(workspaceB)
		for _, store := range []*Store{storeA, storeB} {
			if err := store.EnsureDirs(); err != nil {
				t.Fatalf("Store.EnsureDirs() error = %v", err)
			}
		}

		if err := storeA.Write(memcontract.ScopeGlobal, "prefs.md", mustMemoryContent(t, testMemoryMeta{
			Name: "Shared Preferences",
			Type: memcontract.TypeUser,
		}, "Global signal.\n")); err != nil {
			t.Fatalf("storeA.Write(global) error = %v", err)
		}
		if err := storeA.Write(memcontract.ScopeWorkspace, "project-a.md", mustMemoryContent(t, testMemoryMeta{
			Name: "Workspace A",
			Type: memcontract.TypeProject,
		}, "Workspace A signal.\n")); err != nil {
			t.Fatalf("storeA.Write(workspace) error = %v", err)
		}
		if err := storeB.Write(memcontract.ScopeWorkspace, "project-b.md", mustMemoryContent(t, testMemoryMeta{
			Name: "Workspace B",
			Type: memcontract.TypeProject,
		}, "Workspace B signal.\n")); err != nil {
			t.Fatalf("storeB.Write(workspace) error = %v", err)
		}

		db, err := storeA.catalog.ensureDB(ctx)
		if err != nil {
			t.Fatalf("catalog.ensureDB() error = %v", err)
		}
		globalAt := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
		workspaceAAt := globalAt.Add(time.Minute)
		workspaceBAt := globalAt.Add(2 * time.Minute)
		identityA, err := aghworkspace.EnsureIdentity(ctx, workspaceA)
		if err != nil {
			t.Fatalf("workspace A EnsureIdentity() error = %v", err)
		}
		identityB, err := aghworkspace.EnsureIdentity(ctx, workspaceB)
		if err != nil {
			t.Fatalf("workspace B EnsureIdentity() error = %v", err)
		}
		updates := []struct {
			scope       memcontract.Scope
			workspaceID string
			timestamp   time.Time
		}{
			{scope: memcontract.ScopeGlobal, timestamp: globalAt},
			{scope: memcontract.ScopeWorkspace, workspaceID: identityA.WorkspaceID, timestamp: workspaceAAt},
			{scope: memcontract.ScopeWorkspace, workspaceID: identityB.WorkspaceID, timestamp: workspaceBAt},
		}
		for _, update := range updates {
			if _, err := db.ExecContext(
				ctx,
				`UPDATE memory_events SET ts_ms = ? WHERE scope = ? AND COALESCE(workspace_id, '') = ?`,
				update.timestamp.UTC().UnixNano()/int64(time.Millisecond),
				string(update.scope),
				update.workspaceID,
			); err != nil {
				t.Fatalf("update operation timestamp for %q/%q error = %v", update.scope, update.workspaceID, err)
			}
		}

		stats, err := storeA.HealthStats(ctx, []string{workspaceA})
		if err != nil {
			t.Fatalf("storeA.HealthStats() error = %v", err)
		}
		if got, want := stats.OperationCount, 2; got != want {
			t.Fatalf("memcontract.HealthStats().OperationCount = %d, want %d", got, want)
		}
		if stats.LastOperationAt == nil || !stats.LastOperationAt.Equal(workspaceAAt) {
			t.Fatalf("memcontract.HealthStats().LastOperationAt = %v, want %s", stats.LastOperationAt, workspaceAAt)
		}
	})

	t.Run("Should clamp oversized search limits server-side", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		catalogPath := filepath.Join(baseDir, "agh.db")
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(catalogPath),
		)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}

		for idx := range maxSearchLimit + 5 {
			filename := fmt.Sprintf("shared-%02d.md", idx)
			if err := store.Write(memcontract.ScopeGlobal, filename, mustMemoryContent(t, testMemoryMeta{
				Name:        fmt.Sprintf("Shared signal %02d", idx),
				Description: "Common token across many memories",
				Type:        memcontract.TypeUser,
			}, "Common token appears in every generated memory.\n")); err != nil {
				t.Fatalf("Store.Write(%q) error = %v", filename, err)
			}
		}

		results, err := store.Search(context.Background(), "common token", memcontract.SearchOptions{
			Scope: memcontract.ScopeGlobal,
			Limit: maxSearchLimit + 25,
		})
		if err != nil {
			t.Fatalf("Store.Search() error = %v", err)
		}
		if len(results) != maxSearchLimit {
			t.Fatalf("len(results) = %d, want %d", len(results), maxSearchLimit)
		}
	})

	t.Run("Should index a new workspace even when global catalog rows already exist", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		ctx := context.Background()
		catalogPath := filepath.Join(baseDir, "agh.db")
		globalDir := filepath.Join(baseDir, "global")
		seedWorkspace := filepath.Join(baseDir, "workspace-seed")
		freshWorkspace := filepath.Join(baseDir, "workspace-fresh")

		seedStore := newOpenTestStore(t, globalDir, WithCatalogDatabasePath(catalogPath)).ForWorkspace(seedWorkspace)
		if err := seedStore.EnsureDirs(); err != nil {
			t.Fatalf("seedStore.EnsureDirs() error = %v", err)
		}
		if err := seedStore.Write(memcontract.ScopeGlobal, "prefs.md", mustMemoryContent(t, testMemoryMeta{
			Name:        "Shared Preferences",
			Description: "Global shared signal",
			Type:        memcontract.TypeUser,
		}, "Shared signal is available globally.\n")); err != nil {
			t.Fatalf("seedStore.Write(global) error = %v", err)
		}
		if _, err := seedStore.Reindex(ctx, memcontract.ReindexOptions{Scope: memcontract.ScopeGlobal}); err != nil {
			t.Fatalf("seedStore.Reindex(global) error = %v", err)
		}

		freshStore := newOpenTestStore(t, globalDir, WithCatalogDatabasePath(catalogPath)).ForWorkspace(freshWorkspace)
		if err := freshStore.EnsureDirs(); err != nil {
			t.Fatalf("freshStore.EnsureDirs() error = %v", err)
		}
		if err := freshStore.Write(memcontract.ScopeWorkspace, "project.md", mustMemoryContent(t, testMemoryMeta{
			Name:        "Workspace Plan",
			Description: "Workspace shared signal",
			Type:        memcontract.TypeProject,
		}, "Shared signal is available in the fresh workspace.\n")); err != nil {
			t.Fatalf("freshStore.Write(workspace) error = %v", err)
		}

		results, err := freshStore.Search(
			ctx,
			"shared signal",
			memcontract.SearchOptions{Workspace: freshWorkspace, Limit: 5},
		)
		if err != nil {
			t.Fatalf("freshStore.Search() error = %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("len(results) = %d, want 2; results=%#v", len(results), results)
		}

		scopeCounts := map[memcontract.Scope]int{}
		for _, result := range results {
			scopeCounts[result.Scope.Normalize()]++
		}
		if scopeCounts[memcontract.ScopeGlobal] != 1 || scopeCounts[memcontract.ScopeWorkspace] != 1 {
			t.Fatalf("scopeCounts = %#v, want one global and one workspace result", scopeCounts)
		}

		stats, err := freshStore.HealthStats(ctx, []string{freshWorkspace})
		if err != nil {
			t.Fatalf("freshStore.HealthStats() error = %v", err)
		}
		if stats.IndexedFiles != 2 || stats.OrphanedFiles != 0 || stats.LastReindex == nil {
			t.Fatalf("memcontract.HealthStats() = %#v, want indexed=2 orphaned=0 lastReindex set", stats)
		}
	})

	t.Run("Should not reindex empty synced scopes on subsequent reads", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		catalogPath := filepath.Join(baseDir, "agh.db")
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(catalogPath),
		).ForWorkspace(workspaceRoot)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}

		results, err := store.Search(context.Background(), "auth", memcontract.SearchOptions{
			Workspace: workspaceRoot,
			Limit:     5,
		})
		if err != nil {
			t.Fatalf("Store.Search() error = %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("len(results) = %d, want 0", len(results))
		}

		identity, err := aghworkspace.EnsureIdentity(context.Background(), workspaceRoot)
		if err != nil {
			t.Fatalf("workspace EnsureIdentity() error = %v", err)
		}
		workspaceReady, err := store.catalog.scopeReady(
			context.Background(),
			memcontract.ScopeWorkspace,
			identity.WorkspaceID,
		)
		if err != nil {
			t.Fatalf("catalog.scopeReady(workspace) error = %v", err)
		}
		if !workspaceReady {
			t.Fatal("catalog.scopeReady(workspace) = false, want true after empty reindex")
		}
		globalReady, err := store.catalog.scopeReady(context.Background(), memcontract.ScopeGlobal, "")
		if err != nil {
			t.Fatalf("catalog.scopeReady(global) error = %v", err)
		}
		if !globalReady {
			t.Fatal("catalog.scopeReady(global) = false, want true after empty reindex")
		}

		firstReindex, err := store.catalog.lastReindex(context.Background())
		if err != nil {
			t.Fatalf("catalog.lastReindex() error = %v", err)
		}
		if firstReindex == nil {
			t.Fatal("catalog.lastReindex() = nil, want timestamp after initial reindex")
			return
		}

		stats, err := store.HealthStats(context.Background(), []string{workspaceRoot})
		if err != nil {
			t.Fatalf("Store.HealthStats() error = %v", err)
		}
		if stats.IndexedFiles != 0 || stats.OrphanedFiles != 0 || stats.LastReindex == nil {
			t.Fatalf("memcontract.HealthStats() = %#v, want indexed=0 orphaned=0 lastReindex set", stats)
		}

		secondReindex, err := store.catalog.lastReindex(context.Background())
		if err != nil {
			t.Fatalf("catalog.lastReindex() error = %v", err)
		}
		if secondReindex == nil {
			t.Fatal("catalog.lastReindex() = nil, want timestamp after health check")
			return
		}
		if !secondReindex.Equal(*firstReindex) {
			t.Fatalf(
				"catalog.lastReindex() changed from %s to %s, want empty synced scopes to stay warm",
				firstReindex.Format(time.RFC3339Nano),
				secondReindex.Format(time.RFC3339Nano),
			)
		}
	})
}

func TestStoreConcurrentMutationDerivedState(t *testing.T) {
	t.Run("Should index and log every concurrent workspace write", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		totalWrites := 512
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")),
		).ForWorkspace(workspaceRoot)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}

		sem := make(chan struct{}, 32)
		errCh := make(chan error, totalWrites)
		var wg sync.WaitGroup
		for idx := range totalWrites {
			wg.Go(func() {
				sem <- struct{}{}
				defer func() {
					<-sem
				}()

				filename := fmt.Sprintf("stress-%04d.md", idx)
				content := fmt.Appendf(
					nil,
					"---\nname: Stress %04d\ndescription: concurrent mutation marker\ntype: project\n---\nconcurrent mutation marker item-%04d\n",
					idx,
					idx,
				)
				errCh <- store.Write(memcontract.ScopeWorkspace, filename, content)
			})
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Fatalf("Store.Write(concurrent) error = %v", err)
			}
		}

		stats, err := store.HealthStats(ctx, []string{workspaceRoot})
		if err != nil {
			t.Fatalf("Store.HealthStats() error = %v", err)
		}
		if stats.IndexedFiles != totalWrites || stats.OrphanedFiles != 0 || stats.OperationCount != totalWrites {
			t.Fatalf(
				"memcontract.HealthStats() = %#v, want indexed=%d orphaned=0 operation_count=%d",
				stats,
				totalWrites,
				totalWrites,
			)
		}

		results, err := store.Search(ctx, "concurrent mutation marker", memcontract.SearchOptions{
			Scope:     memcontract.ScopeWorkspace,
			Workspace: workspaceRoot,
			Limit:     maxSearchLimit,
		})
		if err != nil {
			t.Fatalf("Store.Search() error = %v", err)
		}
		if len(results) != maxSearchLimit {
			t.Fatalf("len(results) = %d, want %d", len(results), maxSearchLimit)
		}

		indexBytes, err := os.ReadFile(filepath.Join(store.workspaceDir, indexFilename))
		if err != nil {
			t.Fatalf("os.ReadFile(MEMORY.md) error = %v", err)
		}
		indexLines := strings.Split(strings.TrimSpace(string(indexBytes)), "\n")
		if len(indexLines) != totalWrites {
			t.Fatalf("len(indexLines) = %d, want %d", len(indexLines), totalWrites)
		}
	})
}

func TestStoreOperationHistoryFiltersRedactsBoundsAndPersists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	baseDir := t.TempDir()
	globalDir := filepath.Join(baseDir, "global")
	workspaceRoot := filepath.Join(baseDir, "workspace")
	catalogPath := filepath.Join(baseDir, "agh.db")
	store := newOpenTestStore(t, globalDir, WithCatalogDatabasePath(catalogPath))
	workspaceStore := store.ForWorkspace(workspaceRoot)
	if err := workspaceStore.EnsureDirs(); err != nil {
		t.Fatalf("Store.EnsureDirs() error = %v", err)
	}

	if err := workspaceStore.Write(memcontract.ScopeGlobal, "prefs.md", mustMemoryContent(t, testMemoryMeta{
		Name:        "Global Preferences",
		Description: "Common token lives globally",
		Type:        memcontract.TypeUser,
	}, "Common token is global.\n")); err != nil {
		t.Fatalf("Store.Write(global) error = %v", err)
	}
	if err := workspaceStore.Write(memcontract.ScopeWorkspace, "project.md", mustMemoryContent(t, testMemoryMeta{
		Name:        "Project Memory",
		Description: "Common token lives in the workspace",
		Type:        memcontract.TypeProject,
	}, "Common token is workspace-local.\n")); err != nil {
		t.Fatalf("Store.Write(workspace) error = %v", err)
	}
	identity, err := aghworkspace.EnsureIdentity(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("workspace EnsureIdentity() error = %v", err)
	}

	sinceBeforeSearch := time.Now().Add(-time.Second).UTC()
	results, err := workspaceStore.Search(ctx, "common token=super-secret", memcontract.SearchOptions{
		Workspace: workspaceRoot,
		Limit:     5,
	})
	if err != nil {
		t.Fatalf("Store.Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2; results=%#v", len(results), results)
	}

	workspaceWrites, err := workspaceStore.History(ctx, memcontract.OperationHistoryQuery{
		Scope:     memcontract.ScopeWorkspace,
		Workspace: workspaceRoot,
		Operation: memcontract.OperationWrite,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Store.History(workspace writes) error = %v", err)
	}
	if len(workspaceWrites) != 1 {
		t.Fatalf("len(workspaceWrites) = %d, want 1; records=%#v", len(workspaceWrites), workspaceWrites)
	}
	if workspaceWrites[0].Filename != "project.md" || workspaceWrites[0].Workspace != identity.WorkspaceID {
		t.Fatalf("workspace write record = %#v, want workspace project.md", workspaceWrites[0])
	}

	globalWrites, err := workspaceStore.History(ctx, memcontract.OperationHistoryQuery{
		Scope:     memcontract.ScopeGlobal,
		Operation: memcontract.OperationWrite,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Store.History(global writes) error = %v", err)
	}
	if len(globalWrites) != 1 || globalWrites[0].Filename != "prefs.md" || globalWrites[0].Workspace != "" {
		t.Fatalf("global write records = %#v, want one global prefs.md record", globalWrites)
	}

	searches, err := workspaceStore.History(ctx, memcontract.OperationHistoryQuery{
		Workspace: workspaceRoot,
		Operation: memcontract.OperationSearch,
		Since:     sinceBeforeSearch,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Store.History(searches) error = %v", err)
	}
	if len(searches) != 1 {
		t.Fatalf("len(searches) = %d, want 1; records=%#v", len(searches), searches)
	}
	if strings.Contains(searches[0].Summary, "super-secret") ||
		!strings.Contains(searches[0].Summary, "token=[REDACTED]") {
		t.Fatalf("search summary = %q, want redacted secret token", searches[0].Summary)
	}

	futureHistory, err := workspaceStore.History(ctx, memcontract.OperationHistoryQuery{
		Operation: memcontract.OperationSearch,
		Since:     time.Now().Add(time.Hour).UTC(),
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Store.History(future since) error = %v", err)
	}
	if len(futureHistory) != 0 {
		t.Fatalf("len(futureHistory) = %d, want 0", len(futureHistory))
	}

	for idx := range maxHistoryLimit + 5 {
		if err := workspaceStore.logCatalogEvent(ctx, memcontract.OperationRecord{
			Operation: memcontract.OperationReindex,
			Summary:   fmt.Sprintf("iteration=%d", idx),
		}); err != nil {
			t.Fatalf("logCatalogEvent(%d) error = %v", idx, err)
		}
	}
	bounded, err := workspaceStore.History(ctx, memcontract.OperationHistoryQuery{
		Operation: memcontract.OperationReindex,
		Limit:     maxHistoryLimit + 10,
	})
	if err != nil {
		t.Fatalf("Store.History(bounded) error = %v", err)
	}
	if len(bounded) != maxHistoryLimit {
		t.Fatalf("len(bounded) = %d, want %d", len(bounded), maxHistoryLimit)
	}

	stats, err := workspaceStore.HealthStats(ctx, []string{workspaceRoot})
	if err != nil {
		t.Fatalf("Store.HealthStats() error = %v", err)
	}
	if stats.OperationCount < maxHistoryLimit+8 || stats.LastOperationAt == nil {
		t.Fatalf("memcontract.HealthStats() = %#v, want operation count and last operation", stats)
	}

	reopened := newOpenTestStore(t, globalDir, WithCatalogDatabasePath(catalogPath)).ForWorkspace(workspaceRoot)
	reopenedWrites, err := reopened.History(ctx, memcontract.OperationHistoryQuery{
		Operation: memcontract.OperationWrite,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("reopened.History(writes) error = %v", err)
	}
	if len(reopenedWrites) != 2 {
		t.Fatalf("len(reopenedWrites) = %d, want 2; records=%#v", len(reopenedWrites), reopenedWrites)
	}
}

func TestStoreOperationHistoryIsolatesWorkspaceDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate history by workspace", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		globalDir := filepath.Join(baseDir, "global")
		catalogPath := filepath.Join(baseDir, "agh.db")
		workspaceA := filepath.Join(baseDir, "workspace-a")
		workspaceB := filepath.Join(baseDir, "workspace-b")
		storeA := newOpenTestStore(t, globalDir, WithCatalogDatabasePath(catalogPath)).ForWorkspace(workspaceA)
		storeB := newOpenTestStore(t, globalDir, WithCatalogDatabasePath(catalogPath)).ForWorkspace(workspaceB)
		for _, store := range []*Store{storeA, storeB} {
			if err := store.EnsureDirs(); err != nil {
				t.Fatalf("Store.EnsureDirs() error = %v", err)
			}
		}

		if err := storeA.Write(memcontract.ScopeWorkspace, "project-a.md", mustMemoryContent(t, testMemoryMeta{
			Name: "Workspace A",
			Type: memcontract.TypeProject,
		}, "Alpha workspace signal.\n")); err != nil {
			t.Fatalf("storeA.Write(workspace) error = %v", err)
		}
		if err := storeB.Write(memcontract.ScopeWorkspace, "project-b.md", mustMemoryContent(t, testMemoryMeta{
			Name: "Workspace B",
			Type: memcontract.TypeProject,
		}, "Beta workspace signal.\n")); err != nil {
			t.Fatalf("storeB.Write(workspace) error = %v", err)
		}
		identityA, err := aghworkspace.EnsureIdentity(ctx, workspaceA)
		if err != nil {
			t.Fatalf("workspace A EnsureIdentity() error = %v", err)
		}
		identityB, err := aghworkspace.EnsureIdentity(ctx, workspaceB)
		if err != nil {
			t.Fatalf("workspace B EnsureIdentity() error = %v", err)
		}

		if _, err := storeA.Search(ctx, "alpha signal", memcontract.SearchOptions{Limit: 5}); err != nil {
			t.Fatalf("storeA.Search() error = %v", err)
		}
		if _, err := storeB.Search(ctx, "beta signal", memcontract.SearchOptions{Limit: 5}); err != nil {
			t.Fatalf("storeB.Search() error = %v", err)
		}

		historyA, err := storeA.History(
			ctx,
			memcontract.OperationHistoryQuery{Operation: memcontract.OperationSearch, Limit: 10},
		)
		if err != nil {
			t.Fatalf("storeA.History(searches) error = %v", err)
		}
		if len(historyA) != 1 || historyA[0].Workspace != identityA.WorkspaceID ||
			historyA[0].Scope != memcontract.ScopeWorkspace {
			t.Fatalf("storeA history = %#v, want only workspace A search", historyA)
		}

		historyB, err := storeB.History(
			ctx,
			memcontract.OperationHistoryQuery{Operation: memcontract.OperationSearch, Limit: 10},
		)
		if err != nil {
			t.Fatalf("storeB.History(searches) error = %v", err)
		}
		if len(historyB) != 1 || historyB[0].Workspace != identityB.WorkspaceID ||
			historyB[0].Scope != memcontract.ScopeWorkspace {
			t.Fatalf("storeB history = %#v, want only workspace B search", historyB)
		}
	})
}

func TestStoreSearchTreatsFTSReservedWordsAsLiteralTerms(t *testing.T) {
	t.Run("Should treat FTS reserved words as literal search terms", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		catalogPath := filepath.Join(baseDir, "agh.db")
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(catalogPath),
		).ForWorkspace(workspaceRoot)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		if err := store.Write(memcontract.ScopeGlobal, "operators.md", mustMemoryContent(t, testMemoryMeta{
			Name:        "Reserved Words",
			Description: "Contains literal FTS keywords",
			Type:        memcontract.TypeUser,
		}, "Remember the literal token not in this memory.\n")); err != nil {
			t.Fatalf("Store.Write() error = %v", err)
		}

		results, err := store.Search(
			context.Background(),
			"not",
			memcontract.SearchOptions{Workspace: workspaceRoot, Limit: 5},
		)
		if err != nil {
			t.Fatalf("Store.Search() error = %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("len(results) = %d, want 1; results=%#v", len(results), results)
		}
		if got, want := results[0].Filename, "operators.md"; got != want {
			t.Fatalf("results[0].Filename = %q, want %q", got, want)
		}
	})
}

func TestStoreMutationsStaySuccessfulWhenDerivedSyncFails(t *testing.T) {
	t.Run("Should keep primary mutations successful when derived sync fails", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		catalogPath := filepath.Join(baseDir, "agh.db")

		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(catalogPath),
		).ForWorkspace(workspaceRoot)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		if err := store.CloseCatalog(testutil.Context(t)); err != nil {
			t.Fatalf("Store.CloseCatalog() error = %v", err)
		}

		var logs bytes.Buffer
		store.logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
		content := mustMemoryContent(t, testMemoryMeta{
			Name:        "Prefs",
			Description: "Saved preference",
			Type:        memcontract.TypeUser,
		}, "body\n")

		if err := store.Write(memcontract.ScopeGlobal, "prefs.md", content); err != nil {
			t.Fatalf("Store.Write() error = %v, want primary mutation to succeed", err)
		}
		if _, err := store.Read(memcontract.ScopeGlobal, "prefs.md"); err != nil {
			t.Fatalf("Store.Read() error = %v, want written file present", err)
		}
		if !strings.Contains(logs.String(), "sync derived state failed after mutation") {
			t.Fatalf("logs = %q, want derived sync warning", logs.String())
		}

		logs.Reset()
		if err := store.Delete(memcontract.ScopeGlobal, "prefs.md"); err != nil {
			t.Fatalf("Store.Delete() error = %v, want primary mutation to succeed", err)
		}
		if _, err := store.Read(memcontract.ScopeGlobal, "prefs.md"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Store.Read(deleted) error = %v, want os.ErrNotExist", err)
		}
		if !strings.Contains(logs.String(), "sync derived state failed after mutation") {
			t.Fatalf("logs = %q, want derived sync warning after delete", logs.String())
		}
	})

	t.Run("Should invalidate and repair ready catalogs after write and delete sync failures", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")),
		)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		var logs bytes.Buffer
		store.logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))

		keptContent := mustMemoryContent(t, testMemoryMeta{
			Name: "Kept Memory",
			Type: memcontract.TypeUser,
		}, "kept body\n")
		if err := store.Write(memcontract.ScopeGlobal, "kept.md", keptContent); err != nil {
			t.Fatalf("Store.Write(kept) error = %v", err)
		}
		if _, err := store.CatalogHeaders(ctx, memcontract.ScopeGlobal); err != nil {
			t.Fatalf("Store.CatalogHeaders(seed) error = %v", err)
		}
		assertMemoryCatalogIdentityReady(ctx, t, store, true)

		db, err := store.catalog.ensureDB(ctx)
		if err != nil {
			t.Fatalf("catalog.ensureDB() error = %v", err)
		}
		dropInsertTrigger := installMemoryCatalogAbortTrigger(ctx, t, db, "insert")
		newContent := mustMemoryContent(t, testMemoryMeta{
			Name: "New Memory",
			Type: memcontract.TypeProject,
		}, "new body\n")
		if err := store.Write(memcontract.ScopeGlobal, "new.md", newContent); err != nil {
			t.Fatalf("Store.Write(new) error = %v, want primary mutation success", err)
		}
		if _, err := store.Read(memcontract.ScopeGlobal, "new.md"); err != nil {
			t.Fatalf("Store.Read(new) error = %v, want written source", err)
		}
		assertMemoryCatalogIdentityReady(ctx, t, store, false)
		if !strings.Contains(logs.String(), "sync derived state failed after mutation") {
			t.Fatalf("logs after write = %q, want derived sync warning", logs.String())
		}
		dropInsertTrigger()

		headers, err := store.CatalogHeaders(ctx, memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Store.CatalogHeaders(after write) error = %v", err)
		}
		assertMemoryCatalogFilenames(t, headers, "kept.md", "new.md")
		assertMemoryCatalogIdentityReady(ctx, t, store, true)

		logs.Reset()
		dropDeleteTrigger := installMemoryCatalogAbortTrigger(ctx, t, db, "delete")
		if err := store.Delete(memcontract.ScopeGlobal, "new.md"); err != nil {
			t.Fatalf("Store.Delete(new) error = %v, want primary mutation success", err)
		}
		if _, err := store.Read(memcontract.ScopeGlobal, "new.md"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Store.Read(deleted new) error = %v, want os.ErrNotExist", err)
		}
		assertMemoryCatalogIdentityReady(ctx, t, store, false)
		if !strings.Contains(logs.String(), "sync derived state failed after mutation") {
			t.Fatalf("logs after delete = %q, want derived sync warning", logs.String())
		}
		dropDeleteTrigger()

		headers, err = store.CatalogHeaders(ctx, memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Store.CatalogHeaders(after delete) error = %v", err)
		}
		assertMemoryCatalogFilenames(t, headers, "kept.md")
		assertMemoryCatalogIdentityReady(ctx, t, store, true)

		logs.Reset()
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()
		canceledContent := mustMemoryContent(t, testMemoryMeta{
			Name: "Canceled Sync Memory",
			Type: memcontract.TypeFeedback,
		}, "canceled sync body\n")
		if err := store.writeRaw(
			canceledCtx,
			memcontract.ScopeGlobal,
			"canceled.md",
			canceledContent,
			true,
		); err != nil {
			t.Fatalf("Store.writeRaw(canceled sync) error = %v, want primary mutation success", err)
		}
		if _, err := store.Read(memcontract.ScopeGlobal, "canceled.md"); err != nil {
			t.Fatalf("Store.Read(canceled) error = %v, want written source", err)
		}
		assertMemoryCatalogIdentityReady(ctx, t, store, false)
		if !strings.Contains(logs.String(), "sync derived state failed after mutation") {
			t.Fatalf("logs after canceled sync = %q, want derived sync warning", logs.String())
		}

		headers, err = store.CatalogHeaders(ctx, memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Store.CatalogHeaders(after canceled sync) error = %v", err)
		}
		assertMemoryCatalogFilenames(t, headers, "canceled.md", "kept.md")
		assertMemoryCatalogIdentityReady(ctx, t, store, true)
	})

	t.Run("Should leave a failed derived reset dirty for the next catalog repair", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")),
		)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		content := mustMemoryContent(t, testMemoryMeta{
			Name: "Reset Memory",
			Type: memcontract.TypeReference,
		}, "reset body\n")
		if err := store.Write(memcontract.ScopeGlobal, "reset.md", content); err != nil {
			t.Fatalf("Store.Write(reset) error = %v", err)
		}
		if _, err := store.CatalogHeaders(ctx, memcontract.ScopeGlobal); err != nil {
			t.Fatalf("Store.CatalogHeaders(seed) error = %v", err)
		}
		assertMemoryCatalogIdentityReady(ctx, t, store, true)

		db, err := store.catalog.ensureDB(ctx)
		if err != nil {
			t.Fatalf("catalog.ensureDB() error = %v", err)
		}
		dropInsertTrigger := installMemoryCatalogAbortTrigger(ctx, t, db, "insert")
		if _, err := store.ResetDerived(ctx, memcontract.ReindexOptions{Scope: memcontract.ScopeGlobal}); err == nil {
			t.Fatal("Store.ResetDerived() error = nil, want forced rebuild failure")
		}
		assertMemoryCatalogIdentityReady(ctx, t, store, false)
		var rowCount int
		if err := db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM memory_catalog_entries WHERE scope = ?`,
			string(memcontract.ScopeGlobal),
		).Scan(&rowCount); err != nil {
			t.Fatalf("query cleared catalog row count error = %v", err)
		}
		if rowCount != 0 {
			t.Fatalf("cleared catalog row count = %d, want 0 after failed rebuild", rowCount)
		}
		dropInsertTrigger()

		headers, err := store.CatalogHeaders(ctx, memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Store.CatalogHeaders(repair) error = %v", err)
		}
		assertMemoryCatalogFilenames(t, headers, "reset.md")
		assertMemoryCatalogIdentityReady(ctx, t, store, true)
	})

	t.Run("Should repair a durable dirty catalog after restart and a later mutation", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		globalDir := filepath.Join(baseDir, "global")
		catalogPath := filepath.Join(baseDir, "agh.db")
		store := newOpenTestStore(t, globalDir, WithCatalogDatabasePath(catalogPath))
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		var logs bytes.Buffer
		store.logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))

		seedContent := mustMemoryContent(t, testMemoryMeta{
			Name: "Seed Memory",
			Type: memcontract.TypeUser,
		}, "seed body\n")
		if err := store.Write(memcontract.ScopeGlobal, "seed.md", seedContent); err != nil {
			t.Fatalf("Store.Write(seed) error = %v", err)
		}
		if _, err := store.CatalogHeaders(ctx, memcontract.ScopeGlobal); err != nil {
			t.Fatalf("Store.CatalogHeaders(seed) error = %v", err)
		}
		assertMemoryCatalogIdentityReady(ctx, t, store, true)

		db, err := store.catalog.ensureDB(ctx)
		if err != nil {
			t.Fatalf("catalog.ensureDB() error = %v", err)
		}
		dropInsertTrigger := installMemoryCatalogAbortTrigger(ctx, t, db, "insert")
		dropStateTrigger := installMemoryCatalogAbortTrigger(ctx, t, db, "state_delete")
		lostContent := mustMemoryContent(t, testMemoryMeta{
			Name: "Recovered Memory",
			Type: memcontract.TypeProject,
		}, "recovered body\n")
		if err := store.Write(memcontract.ScopeGlobal, "recovered.md", lostContent); err != nil {
			t.Fatalf("Store.Write(recovered) error = %v, want primary mutation success", err)
		}
		if _, err := store.Read(memcontract.ScopeGlobal, "recovered.md"); err != nil {
			t.Fatalf("Store.Read(recovered) error = %v, want written source", err)
		}
		assertMemoryCatalogIdentityReady(ctx, t, store, true)
		dirty, err := store.catalogSourceDirty(memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Store.catalogSourceDirty() error = %v", err)
		}
		if !dirty {
			t.Fatal("Store.catalogSourceDirty() = false, want durable marker after double failure")
		}
		markerPath, err := store.catalogDirtyMarkerPath(memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Store.catalogDirtyMarkerPath() error = %v", err)
		}
		markerBefore, err := os.ReadFile(markerPath)
		if err != nil {
			t.Fatalf("ReadFile(catalog dirty marker) error = %v", err)
		}
		reservedContent := mustMemoryContent(t, testMemoryMeta{
			Name: "Reserved Marker Collision",
			Type: memcontract.TypeUser,
		}, "reserved body\n")
		for _, reservedFilename := range []string{
			catalogDirtyMarkerFilename,
			strings.ToUpper(catalogDirtyMarkerFilename),
		} {
			writeErr := store.Write(memcontract.ScopeGlobal, reservedFilename, reservedContent)
			if !errors.Is(writeErr, ErrValidation) {
				t.Fatalf("Store.Write(%q) error = %v, want ErrValidation", reservedFilename, writeErr)
			}
			deleteErr := store.Delete(memcontract.ScopeGlobal, reservedFilename)
			if !errors.Is(deleteErr, ErrValidation) {
				t.Fatalf("Store.Delete(%q) error = %v, want ErrValidation", reservedFilename, deleteErr)
			}
		}
		markerAfter, err := os.ReadFile(markerPath)
		if err != nil {
			t.Fatalf("ReadFile(catalog dirty marker after rejected mutations) error = %v", err)
		}
		if !bytes.Equal(markerAfter, markerBefore) {
			t.Fatalf(
				"catalog dirty marker changed after rejected reserved mutations: got %q want %q",
				markerAfter,
				markerBefore,
			)
		}
		if !strings.Contains(logs.String(), "catalog identity invalidation failed after derived sync failure") {
			t.Fatalf("logs = %q, want secondary invalidation warning", logs.String())
		}
		probe := newOpenTestStore(t, globalDir, WithCatalogDatabasePath(catalogPath))
		if _, err := probe.CatalogHeaders(ctx, memcontract.ScopeGlobal); err == nil {
			t.Fatal("restarted Store.CatalogHeaders() error = nil, want dirty marker to force a rebuild")
		}
		dropInsertTrigger()
		dropStateTrigger()

		restarted := newOpenTestStore(t, globalDir, WithCatalogDatabasePath(catalogPath))
		if err := restarted.EnsureDirs(); err != nil {
			t.Fatalf("restarted Store.EnsureDirs() error = %v", err)
		}
		laterContent := mustMemoryContent(t, testMemoryMeta{
			Name: "Later Memory",
			Type: memcontract.TypeFeedback,
		}, "later body\n")
		if err := restarted.Write(memcontract.ScopeGlobal, "later.md", laterContent); err != nil {
			t.Fatalf("restarted Store.Write(later) error = %v", err)
		}
		dirty, err = restarted.catalogSourceDirty(memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("restarted Store.catalogSourceDirty() error = %v", err)
		}
		if dirty {
			t.Fatal("restarted Store.catalogSourceDirty() = true after successful full repair")
		}

		headers, err := restarted.CatalogHeaders(ctx, memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("restarted Store.CatalogHeaders() error = %v", err)
		}
		assertMemoryCatalogFilenames(t, headers, "later.md", "recovered.md", "seed.md")
		assertMemoryCatalogIdentityReady(ctx, t, restarted, true)
	})
}

func installMemoryCatalogAbortTrigger(
	ctx context.Context,
	t *testing.T,
	db *sql.DB,
	operation string,
) func() {
	t.Helper()

	var createStatement string
	var dropStatement string
	switch operation {
	case "insert":
		createStatement = `CREATE TRIGGER memory_catalog_test_abort_insert
			BEFORE INSERT ON memory_catalog_entries
			BEGIN
				SELECT RAISE(ABORT, 'forced memory catalog insert failure');
			END`
		dropStatement = `DROP TRIGGER IF EXISTS memory_catalog_test_abort_insert`
	case "delete":
		createStatement = `CREATE TRIGGER memory_catalog_test_abort_delete
			BEFORE DELETE ON memory_catalog_entries
			BEGIN
				SELECT RAISE(ABORT, 'forced memory catalog delete failure');
			END`
		dropStatement = `DROP TRIGGER IF EXISTS memory_catalog_test_abort_delete`
	case "state_delete":
		createStatement = `CREATE TRIGGER memory_catalog_test_abort_state_delete
			BEFORE DELETE ON memory_catalog_state
			BEGIN
				SELECT RAISE(ABORT, 'forced memory catalog state delete failure');
			END`
		dropStatement = `DROP TRIGGER IF EXISTS memory_catalog_test_abort_state_delete`
	default:
		t.Fatalf("unsupported catalog abort trigger operation %q", operation)
	}
	if _, err := db.ExecContext(ctx, createStatement); err != nil {
		t.Fatalf("install catalog %s abort trigger error = %v", operation, err)
	}

	drop := func() {
		t.Helper()
		if _, err := db.ExecContext(ctx, dropStatement); err != nil {
			t.Errorf("drop catalog %s abort trigger error = %v", operation, err)
		}
	}
	t.Cleanup(drop)
	return drop
}

func assertMemoryCatalogIdentityReady(ctx context.Context, t *testing.T, store *Store, want bool) {
	t.Helper()

	ready, err := store.catalog.identityReady(
		ctx,
		newCatalogIdentity(memcontract.ScopeGlobal, "", "", ""),
	)
	if err != nil {
		t.Fatalf("catalog.identityReady(global) error = %v", err)
	}
	if ready != want {
		t.Fatalf("catalog.identityReady(global) = %t, want %t", ready, want)
	}
}

func assertMemoryCatalogFilenames(t *testing.T, headers []memcontract.Header, want ...string) {
	t.Helper()

	got := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		got[header.Filename] = struct{}{}
	}
	if len(got) != len(want) {
		t.Fatalf("catalog filenames = %#v, want %#v", got, want)
	}
	for _, filename := range want {
		if _, ok := got[filename]; !ok {
			t.Fatalf("catalog filenames = %#v, want %#v", got, want)
		}
	}
}

func TestStoreEnsureDirs(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := newOpenTestStore(t, filepath.Join(baseDir, "home", "memory")).ForWorkspace(
		filepath.Join(baseDir, "workspace"),
	)

	if err := store.EnsureDirs(); err != nil {
		t.Fatalf("Store.EnsureDirs() error = %v", err)
	}

	for _, dir := range []string{store.globalDir, store.workspaceDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("os.Stat(%q) error = %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", dir)
		}
	}

	baseStore := newOpenTestStore(t, filepath.Join(baseDir, "only-global"))
	if err := baseStore.EnsureDirs(); err != nil {
		t.Fatalf("Store.EnsureDirs() with global-only store error = %v", err)
	}
}

func TestWorkspaceMemoryDirUsesWorkspaceRoot(t *testing.T) {
	t.Run("Should place workspace memory under the workspace identity directory", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		want := filepath.Join(workspaceRoot, ".agh", "memory")
		if got := workspaceMemoryDir(workspaceRoot); got != want {
			t.Fatalf("workspaceMemoryDir(%q) = %q, want %q", workspaceRoot, got, want)
		}
	})
}

func TestStoreNormalizesExplicitWorkspacePaths(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should search workspace memories when the workspace option points at the workspace root",
		func(t *testing.T) {
			t.Parallel()

			baseDir := t.TempDir()
			workspaceRoot := filepath.Join(baseDir, "workspace")
			store := newOpenTestStore(t,
				filepath.Join(baseDir, "global"),
				WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")),
			).ForWorkspace(workspaceRoot)
			if err := store.EnsureDirs(); err != nil {
				t.Fatalf("Store.EnsureDirs() error = %v", err)
			}
			if err := store.Write(memcontract.ScopeWorkspace, "project.md", mustMemoryContent(t, testMemoryMeta{
				Name:        "Workspace Search",
				Description: "Normalize explicit workspace paths",
				Type:        memcontract.TypeProject,
			}, "Unique workspace signal for normalization coverage.\n")); err != nil {
				t.Fatalf("Store.Write(workspace) error = %v", err)
			}

			results, err := store.Search(context.Background(), "unique workspace signal", memcontract.SearchOptions{
				Scope:     memcontract.ScopeWorkspace,
				Workspace: workspaceRoot,
				Limit:     5,
			})
			if err != nil {
				t.Fatalf("Store.Search() error = %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("len(results) = %d, want 1; results=%#v", len(results), results)
			}
			if results[0].Scope != memcontract.ScopeWorkspace {
				t.Fatalf("results[0].Scope = %q, want %q", results[0].Scope, memcontract.ScopeWorkspace)
			}
			if !aghworkspace.IsWorkspaceID(results[0].Workspace) {
				t.Fatalf("results[0].Workspace = %q, want workspace_id", results[0].Workspace)
			}
		},
	)

	t.Run("Should include workspace memories in health stats when given the workspace root", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")),
		).ForWorkspace(workspaceRoot)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		if err := store.Write(memcontract.ScopeWorkspace, "project.md", mustMemoryContent(t, testMemoryMeta{
			Name:        "Workspace Health",
			Description: "Normalize health stats workspace filters",
			Type:        memcontract.TypeProject,
		}, "Workspace health stats should use the canonical workspace root.\n")); err != nil {
			t.Fatalf("Store.Write(workspace) error = %v", err)
		}

		stats, err := store.HealthStats(context.Background(), []string{workspaceRoot})
		if err != nil {
			t.Fatalf("Store.HealthStats() error = %v", err)
		}
		if stats.IndexedFiles != 1 || stats.OrphanedFiles != 0 || stats.LastReindex == nil {
			t.Fatalf("memcontract.HealthStats() = %#v, want indexed=1 orphaned=0 lastReindex set", stats)
		}
	})
}

func TestStoreRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		run     func(*testStoreEnv) error
		wantErr string
		verify  func(*testing.T, *testStoreEnv)
	}{
		{
			name: "invalid scope on scan",
			run: func(env *testStoreEnv) error {
				_, err := env.store.Scan(memcontract.Scope("sideways"))
				return err
			},
			wantErr: `unsupported scope "sideways"`,
		},
		{
			name: "invalid scope on load index",
			run: func(env *testStoreEnv) error {
				_, _, err := env.store.LoadIndex(memcontract.Scope("sideways"))
				return err
			},
			wantErr: `unsupported scope "sideways"`,
		},
		{
			name: "missing workspace directory",
			run: func(env *testStoreEnv) error {
				_, err := newOpenTestStore(t, env.store.globalDir).Scan(memcontract.ScopeWorkspace)
				return err
			},
			wantErr: "workspace directory is required",
		},
		{
			name: "path traversal filename on read",
			run: func(env *testStoreEnv) error {
				_, err := env.store.Read(memcontract.ScopeGlobal, "nested/file.md")
				return err
			},
			wantErr: "must not include path separators",
		},
		{
			name: "empty filename on delete",
			run: func(env *testStoreEnv) error {
				return env.store.Delete(memcontract.ScopeGlobal, " ")
			},
			wantErr: "filename is required",
		},
		{
			name: "normalized memory type",
			run: func(env *testStoreEnv) error {
				return env.store.Write(memcontract.ScopeGlobal, "normalized.md", []byte(`---
name: Normalized Type
type: "  PROJECT "
---
Body
`))
			},
			wantErr: "",
			verify: func(t *testing.T, env *testStoreEnv) {
				t.Helper()

				headers, err := env.store.Scan(memcontract.ScopeGlobal)
				if err != nil {
					t.Fatalf("Store.Scan() error = %v", err)
				}
				if got, want := len(headers), 1; got != want {
					t.Fatalf("len(headers) = %d, want %d", got, want)
				}
				if headers[0].Type != memcontract.TypeProject {
					t.Fatalf("headers[0].Type = %q, want %q", headers[0].Type, memcontract.TypeProject)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := newTestStoreEnv(t)
			err := tt.run(env)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("operation error = %v, want nil", err)
				}
				if tt.verify != nil {
					tt.verify(t, env)
				}
				return
			}
			if err == nil {
				t.Fatal("operation error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("operation error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestStoreScanMissingDirectoryReturnsEmpty(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := newOpenTestStore(t, filepath.Join(baseDir, "global")).ForWorkspace(filepath.Join(baseDir, "workspace"))

	headers, err := store.Scan(memcontract.ScopeWorkspace)
	if err != nil {
		t.Fatalf("Store.Scan() error = %v", err)
	}
	if len(headers) != 0 {
		t.Fatalf("len(headers) = %d, want 0", len(headers))
	}
}

func TestStalenessHelpers(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("UTC-3", -3*60*60)
	now := time.Date(2026, 4, 4, 10, 0, 0, 0, location)

	today := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, location)
	yesterday := today.Add(-24 * time.Hour)
	threeDaysAgo := today.Add(-72 * time.Hour)

	testCases := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "Should return zero days for today",
			run: func(t *testing.T) {
				t.Parallel()
				if got := ageDays(today, now); got != 0 {
					t.Fatalf("ageDays(today) = %d, want 0", got)
				}
			},
		},
		{
			name: "Should return one day for yesterday",
			run: func(t *testing.T) {
				t.Parallel()
				if got := ageDays(yesterday, now); got != 1 {
					t.Fatalf("ageDays(yesterday) = %d, want 1", got)
				}
			},
		},
		{
			name: "Should render today age text",
			run: func(t *testing.T) {
				t.Parallel()
				if got := ageText(today, now); got != "today" {
					t.Fatalf("ageText(today) = %q, want %q", got, "today")
				}
			},
		},
		{
			name: "Should render yesterday age text",
			run: func(t *testing.T) {
				t.Parallel()
				if got := ageText(yesterday, now); got != "yesterday" {
					t.Fatalf("ageText(yesterday) = %q, want %q", got, "yesterday")
				}
			},
		},
		{
			name: "Should render multi-day age text",
			run: func(t *testing.T) {
				t.Parallel()
				if got := ageText(threeDaysAgo, now); got != "3 days ago" {
					t.Fatalf("ageText(threeDaysAgo) = %q, want %q", got, "3 days ago")
				}
			},
		},
		{
			name: "Should omit freshness warning for today",
			run: func(t *testing.T) {
				t.Parallel()
				if got := freshnessWarning(today, now); got != "" {
					t.Fatalf("freshnessWarning(today) = %q, want empty", got)
				}
			},
		},
		{
			name: "Should omit freshness warning for yesterday",
			run: func(t *testing.T) {
				t.Parallel()
				if got := freshnessWarning(yesterday, now); got != "" {
					t.Fatalf("freshnessWarning(yesterday) = %q, want empty", got)
				}
			},
		},
		{
			name: "Should warn for stale memories",
			run: func(t *testing.T) {
				t.Parallel()
				if got := freshnessWarning(threeDaysAgo, now); !strings.Contains(got, "3 days old") {
					t.Fatalf("freshnessWarning(threeDaysAgo) = %q, want age caveat", got)
				}
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestStoreExists(t *testing.T) {
	t.Parallel()

	env := newTestStoreEnv(t)
	content := mustMemoryContent(t, testMemoryMeta{
		Name:        "User Memory",
		Description: "desc",
		Type:        memcontract.TypeUser,
	}, "hello")
	if err := env.store.Write(memcontract.ScopeWorkspace, "exists.md", content); err != nil {
		t.Fatalf("Store.Write() error = %v", err)
	}

	exists, err := env.store.Exists(memcontract.ScopeWorkspace, "exists.md")
	if err != nil {
		t.Fatalf("Store.Exists(exists.md) error = %v", err)
	}
	if !exists {
		t.Fatal("Store.Exists(exists.md) = false, want true")
	}

	missing, err := env.store.Exists(memcontract.ScopeWorkspace, "missing.md")
	if err != nil {
		t.Fatalf("Store.Exists(missing.md) error = %v", err)
	}
	if missing {
		t.Fatal("Store.Exists(missing.md) = true, want false")
	}
}

type testStoreEnv struct {
	store *Store
}

func newTestStoreEnv(t *testing.T) *testStoreEnv {
	t.Helper()

	baseDir := t.TempDir()
	workspaceRoot := filepath.Join(baseDir, "workspace")
	store := newOpenTestStore(t, filepath.Join(baseDir, "global")).ForWorkspace(workspaceRoot)
	if err := store.EnsureDirs(); err != nil {
		t.Fatalf("Store.EnsureDirs() error = %v", err)
	}

	return &testStoreEnv{store: store}
}

func mustMemoryContent(t *testing.T, meta testMemoryMeta, body string) []byte {
	t.Helper()

	metaBytes, err := yaml.Marshal(meta)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	return []byte("---\n" + strings.TrimRight(string(metaBytes), "\n") + "\n---\n" + body)
}

func writeIndexFixtures(t *testing.T, dir string, indexContent string) {
	t.Helper()

	baseModTime := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	for line := range strings.SplitSeq(indexContent, "\n") {
		filename, meta, ok := parseIndexFixture(line)
		if !ok {
			continue
		}
		path := filepath.Join(dir, filename)
		doc := mustMemoryContent(t, meta, "body\n")
		if err := os.WriteFile(path, doc, filePerm); err != nil {
			t.Fatalf("write fixture %q: %v", filename, err)
		}
		if err := os.Chtimes(path, baseModTime, baseModTime); err != nil {
			t.Fatalf("os.Chtimes(%q) error = %v", path, err)
		}
	}
}

func parseIndexFixture(line string) (string, testMemoryMeta, bool) {
	filename, ok := firstMarkdownLinkTarget(line)
	if !ok {
		return "", testMemoryMeta{}, false
	}

	labelStart := strings.Index(line, "[")
	labelEnd := strings.Index(line, "](")
	targetEnd := strings.LastIndex(line, ")")
	if labelStart < 0 || labelEnd <= labelStart || targetEnd < 0 {
		return "", testMemoryMeta{}, false
	}

	name := strings.TrimSpace(line[labelStart+1 : labelEnd])
	description := ""
	if targetEnd+1 < len(line) {
		description = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line[targetEnd+1:]), "-"))
	}

	return filename, testMemoryMeta{
		Name:        name,
		Description: description,
		Type:        memcontract.TypeUser,
	}, true
}
