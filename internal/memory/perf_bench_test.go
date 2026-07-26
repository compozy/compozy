package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/goccy/go-yaml"
)

func BenchmarkStoreScanCappedWorkspace(b *testing.B) {
	env := newBenchmarkStoreEnv(b)
	base := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

	for idx := range 512 {
		filename := fmt.Sprintf("%03d.md", idx)
		payload := mustBenchmarkMemoryContent(b, testMemoryMeta{
			Name:        fmt.Sprintf("Memory %03d", idx),
			Description: "Benchmark memory",
			Type:        memcontract.TypeProject,
		}, "Benchmark body\n")
		if err := env.store.Write(memcontract.ScopeWorkspace, filename, payload); err != nil {
			b.Fatalf("Store.Write(%q) error = %v", filename, err)
		}

		path, err := env.store.pathFor(memcontract.ScopeWorkspace, filename)
		if err != nil {
			b.Fatalf("pathFor(%q) error = %v", filename, err)
		}
		modTime := base.Add(time.Duration(idx) * time.Second)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			b.Fatalf("os.Chtimes(%q) error = %v", path, err)
		}
	}

	for b.Loop() {
		headers, err := env.store.Scan(memcontract.ScopeWorkspace)
		if err != nil {
			b.Fatalf("Store.Scan() error = %v", err)
		}
		if len(headers) != maxScanEntries {
			b.Fatalf("len(headers) = %d, want %d", len(headers), maxScanEntries)
		}
	}
}

func BenchmarkAssemblerPromptSectionDualIndex(b *testing.B) {
	env := newBenchmarkStoreEnv(b)
	globalIndex := filepath.Join(env.store.globalDir, indexFilename)
	workspaceIndex := filepath.Join(env.store.workspaceDir, indexFilename)

	if err := os.WriteFile(globalIndex, benchmarkIndexContent("global"), filePerm); err != nil {
		b.Fatalf("write global index: %v", err)
	}
	if err := os.WriteFile(workspaceIndex, benchmarkIndexContent("workspace"), filePerm); err != nil {
		b.Fatalf("write workspace index: %v", err)
	}

	assembler := NewAssembler(env.store)
	workspace := &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{RootDir: filepath.Dir(env.store.workspaceDir)},
	}
	ctx := context.Background()

	for b.Loop() {
		section, err := assembler.PromptSection(ctx, workspace)
		if err != nil {
			b.Fatalf("PromptSection() error = %v", err)
		}
		if section == "" {
			b.Fatal("PromptSection() = empty, want content")
		}
	}
}

func BenchmarkScanCompletedSessionsSince(b *testing.B) {
	sessionsDir := b.TempDir()
	since := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	service := NewService(WithSessionsDir(sessionsDir))

	for idx := range 400 {
		stoppedAt := since.Add(time.Duration(idx) * time.Minute)
		writeBenchmarkSessionMeta(b, sessionsDir, fmt.Sprintf("session-%03d", idx), persistedSessionMetadata{
			StoppedAt: new(stoppedAt),
		})
	}

	for b.Loop() {
		count, err := service.scanCompletedSessionsSince(since)
		if err != nil {
			b.Fatalf("scanCompletedSessionsSince() error = %v", err)
		}
		if count != 400 {
			b.Fatalf("count = %d, want 400", count)
		}
	}
}

func benchmarkIndexContent(scope string) []byte {
	lines := make([]byte, 0, 16*defaultIndexLines)
	for idx := range defaultIndexLines {
		lines = append(
			lines,
			fmt.Sprintf("- [%s %03d](%s-%03d.md) - durable benchmark entry\n", scope, idx, scope, idx)...)
	}
	return lines
}

func newBenchmarkStoreEnv(b *testing.B) *testStoreEnv {
	b.Helper()

	baseDir := b.TempDir()
	workspaceRoot := filepath.Join(baseDir, "workspace")
	store := NewStore(filepath.Join(baseDir, "global")).ForWorkspace(workspaceRoot)
	if err := store.EnsureDirs(); err != nil {
		b.Fatalf("Store.EnsureDirs() error = %v", err)
	}

	return &testStoreEnv{store: store}
}

func mustBenchmarkMemoryContent(b *testing.B, meta testMemoryMeta, body string) []byte {
	b.Helper()

	metaBytes, err := yaml.Marshal(meta)
	if err != nil {
		b.Fatalf("yaml.Marshal() error = %v", err)
	}

	return []byte("---\n" + strings.TrimRight(string(metaBytes), "\n") + "\n---\n" + body)
}

func writeBenchmarkSessionMeta(b *testing.B, sessionsDir string, sessionID string, meta persistedSessionMetadata) {
	b.Helper()

	sessionDir := filepath.Join(sessionsDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		b.Fatalf("os.MkdirAll(%q) error = %v", sessionDir, err)
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		b.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "meta.json"), payload, 0o644); err != nil {
		b.Fatalf("os.WriteFile(meta.json) error = %v", err)
	}
}
