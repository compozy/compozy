package extensionpkg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func BenchmarkDecodeHostAPIParamsTaskCreate(b *testing.B) {
	b.ReportAllocs()

	raw := json.RawMessage(fmt.Sprintf(`{
		"id":"task-bench",
		"identifier":"bench-task",
		"scope":"workspace",
		"workspace":"ws-bench",
		"network_participation":{"mode":"live","channel_strategy":"named","channel_id":"agent/bench"},
		"title":"Benchmark task",
		"description":"Benchmark payload decode",
		"metadata":{"body":"%s","labels":["alpha","beta","gamma"]}
	}`, extensionBenchmarkText(512)))

	for b.Loop() {
		var params hostAPITaskCreateParams
		if err := decodeHostAPIParams(raw, &params); err != nil {
			b.Fatalf("decodeHostAPIParams() error = %v", err)
		}
		if params.Title != "Benchmark task" {
			b.Fatalf("params.Title = %q, want %q", params.Title, "Benchmark task")
		}
	}
}

func BenchmarkTaskSummaryPayloadsFromSummaries(b *testing.B) {
	b.ReportAllocs()

	summaries := extensionBenchmarkTaskSummaries(512)

	for b.Loop() {
		payloads := taskSummaryPayloadsFromSummaries(summaries)
		if len(payloads) != len(summaries) {
			b.Fatalf("len(payloads) = %d, want %d", len(payloads), len(summaries))
		}
		if payloads[len(payloads)-1].ID == "" {
			b.Fatal("last payload id is empty")
		}
	}
}

func BenchmarkTaskRunPayloadsFromRuns(b *testing.B) {
	b.ReportAllocs()

	runs := extensionBenchmarkTaskRuns(256)

	for b.Loop() {
		payloads := taskRunPayloadsFromRuns(runs)
		if len(payloads) != len(runs) {
			b.Fatalf("len(payloads) = %d, want %d", len(payloads), len(runs))
		}
		if len(payloads[len(payloads)-1].Result) == 0 {
			b.Fatal("last payload result is empty")
		}
	}
}

func BenchmarkExtensionToolProviderListAndResolve(b *testing.B) {
	b.ReportAllocs()

	registry, targetID := extensionBenchmarkToolRegistry(b, 16)
	provider, err := NewExtensionToolProvider(registry, func() ExtensionToolRuntime {
		return nil
	})
	if err != nil {
		b.Fatalf("NewExtensionToolProvider() error = %v", err)
	}
	ctx := testutil.Context(b)
	scope := toolspkg.Scope{Operator: true}

	for b.Loop() {
		descriptors, err := provider.List(ctx, scope)
		if err != nil {
			b.Fatalf("List() error = %v", err)
		}
		if len(descriptors) != 16 {
			b.Fatalf("len(descriptors) = %d, want 16", len(descriptors))
		}
		if _, found, err := provider.Resolve(ctx, scope, targetID); err != nil || !found {
			b.Fatalf("Resolve(%q) = found %v, error %v; want found", targetID, found, err)
		}
	}
}

func extensionBenchmarkToolRegistry(b *testing.B, count int) (*Registry, toolspkg.ToolID) {
	b.Helper()

	dbPath := filepath.Join(b.TempDir(), "agh-extension-tools.db")
	db, err := store.OpenSQLiteDatabase(testutil.Context(b), dbPath, func(ctx context.Context, db *sql.DB) error {
		return store.Apply(ctx, db, globaldb.MigrationStream())
	})
	if err != nil {
		b.Fatalf("OpenSQLiteDatabase() error = %v", err)
	}
	b.Cleanup(func() {
		if err := db.Close(); err != nil {
			b.Fatalf("db.Close() error = %v", err)
		}
	})

	registry := NewRegistry(db)
	var targetID toolspkg.ToolID
	for i := range count {
		name := fmt.Sprintf("bench-tool-%02d", i)
		dir := filepath.Join(b.TempDir(), name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
		}
		if err := os.WriteFile(
			filepath.Join(dir, manifestJSONFileName),
			[]byte(extensionToolManifestJSON(name, "fake-extension", nil, nil, true)),
			0o600,
		); err != nil {
			b.Fatalf("os.WriteFile(%q manifest) error = %v", name, err)
		}
		manifest, err := LoadManifest(dir)
		if err != nil {
			b.Fatalf("LoadManifest(%q) error = %v", dir, err)
		}
		checksum, err := ComputeDirectoryChecksum(dir)
		if err != nil {
			b.Fatalf("ComputeDirectoryChecksum(%q) error = %v", dir, err)
		}
		if err := registry.Install(manifest, dir, checksum); err != nil {
			b.Fatalf("Install(%q) error = %v", name, err)
		}
		descriptors, err := ResolveManifestToolDescriptors(manifest)
		if err != nil {
			b.Fatalf("ResolveManifestToolDescriptors(%q) error = %v", name, err)
		}
		if len(descriptors) != 1 {
			b.Fatalf("len(descriptors for %q) = %d, want 1", name, len(descriptors))
		}
		targetID = descriptors[0].Tool.ID
	}
	return registry, targetID
}

func extensionBenchmarkTaskSummaries(count int) []taskpkg.Summary {
	summaries := make([]taskpkg.Summary, 0, count)
	now := time.Unix(1_700_000_000, 0).UTC()
	for i := range count {
		summaries = append(summaries, taskpkg.Summary{
			ID:           fmt.Sprintf("task-%03d", i),
			Identifier:   fmt.Sprintf("bench-%03d", i),
			Scope:        taskpkg.ScopeWorkspace,
			WorkspaceID:  "ws-bench",
			ParentTaskID: fmt.Sprintf("parent-%03d", i%8),
			Title:        fmt.Sprintf("Benchmark task %03d", i),
			Status:       taskpkg.TaskStatusReady,
			Owner: &taskpkg.Ownership{
				Kind: taskpkg.OwnerKindExtension,
				Ref:  fmt.Sprintf("owner-%03d", i%4),
			},
			CreatedBy: taskpkg.ActorIdentity{
				Kind: taskpkg.ActorKindExtension,
				Ref:  "bench-ext",
			},
			Origin: taskpkg.Origin{
				Kind: taskpkg.OriginKindExtension,
				Ref:  "bench-ext",
			},
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			UpdatedAt: now.Add(time.Duration(i+1) * time.Second),
			ClosedAt:  time.Time{},
		})
	}
	return summaries
}

func extensionBenchmarkTaskRuns(count int) []taskpkg.Run {
	runs := make([]taskpkg.Run, 0, count)
	now := time.Unix(1_700_000_000, 0).UTC()
	result := json.RawMessage(
		fmt.Sprintf(`{"summary":%q,"scores":[1,2,3,4],"ok":true}`, extensionBenchmarkText(1024)),
	)
	for i := range count {
		runs = append(runs, taskpkg.Run{
			ID:             fmt.Sprintf("run-%03d", i),
			TaskID:         fmt.Sprintf("task-%03d", i),
			Status:         taskpkg.TaskRunStatusRunning,
			Attempt:        int32(i%3 + 1),
			ClaimedBy:      extensionBenchmarkClaimedBy(i),
			SessionID:      fmt.Sprintf("session-%03d", i),
			Origin:         taskpkg.Origin{Kind: taskpkg.OriginKindExtension, Ref: "bench-ext"},
			IdempotencyKey: fmt.Sprintf("idem-%03d", i),
			RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: participation.Spec{
				Version:         participation.SpecVersion,
				Mode:            participation.ModeLive,
				ChannelStrategy: participation.StrategyNamed,
				ChannelID:       "agent/bench",
				Source:          participation.SourceExplicitRequest,
			}},
			QueuedAt:  now.Add(time.Duration(i) * time.Second),
			ClaimedAt: now.Add(time.Duration(i+1) * time.Second),
			StartedAt: now.Add(time.Duration(i+2) * time.Second),
			EndedAt:   time.Time{},
			Error:     "",
			Result:    append(json.RawMessage(nil), result...),
		})
	}
	return runs
}

func extensionBenchmarkClaimedBy(i int) *taskpkg.ActorIdentity {
	if i%5 == 0 {
		return nil
	}
	return &taskpkg.ActorIdentity{
		Kind: taskpkg.ActorKindExtension,
		Ref:  fmt.Sprintf("bench-ext-%d", i%7),
	}
}

func extensionBenchmarkText(size int) string {
	if size <= 0 {
		return ""
	}
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = byte('a' + (i % 26))
	}
	return string(buf)
}
