package memstore_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/memory/provider/local/memstore"
	"github.com/compozy/agh/internal/testutil"
	"github.com/goccy/go-yaml"
)

func TestAdapter(t *testing.T) {
	t.Run("Should expose store prompt recall write and agent binding operations", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		baseDir := t.TempDir()
		workspaceRoot := baseDir + "/workspace"
		store := memory.NewStore(
			baseDir+"/agh-home/memory",
			memory.WithCatalogDatabasePath(baseDir+"/agh.db"),
		).ForWorkspace(workspaceRoot)
		openAdapterTestCatalog(t, store)
		adapter := memstore.New(store)
		if err := adapter.EnsureDirs(); err != nil {
			t.Fatalf("Adapter.EnsureDirs() error = %v", err)
		}
		if err := store.Write(memcontract.ScopeGlobal, "project_provider.md", adapterPayload(t)); err != nil {
			t.Fatalf("Store.Write(global) error = %v", err)
		}
		content, truncated, err := adapter.LoadPromptIndex(memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Adapter.LoadPromptIndex() error = %v", err)
		}
		if truncated {
			t.Fatal("Adapter.LoadPromptIndex() truncated = true, want false")
		}
		if !strings.Contains(content, "Provider Adapter") {
			t.Fatalf("Adapter.LoadPromptIndex() = %q, want stored memory", content)
		}
		headers, err := adapter.List(memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Adapter.List() error = %v", err)
		}
		if len(headers) != 1 {
			t.Fatalf("Adapter.List() headers = %d, want 1", len(headers))
		}
		recalled, err := adapter.Recall(ctx, memcontract.Query{
			QueryText: "provider adapter recall",
		}, memcontract.RecallOptions{TopK: 3})
		if err != nil {
			t.Fatalf("Adapter.Recall() error = %v", err)
		}
		if len(recalled.Blocks) == 0 {
			t.Fatal("Adapter.Recall() returned no blocks")
		}
		decision := memcontract.Decision{
			ID:             "dec_adapter",
			CandidateHash:  "hash_adapter",
			IdempotencyKey: "key_adapter",
			Op:             memcontract.OpAdd,
			TargetFilename: "project_adapter_write.md",
			Frontmatter: memcontract.Header{
				Name:  "Adapter Write",
				Type:  memcontract.TypeProject,
				Scope: memcontract.ScopeGlobal,
			},
			PostContent:     string(adapterWritePayload(t)),
			PostContentHash: "hash_content",
			Confidence:      1,
			Source:          memcontract.SourceRule,
			DecidedAt:       time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC),
		}
		if err := adapter.ApplyDecision(ctx, decision); err != nil {
			t.Fatalf("Adapter.ApplyDecision() error = %v", err)
		}
		got, err := store.Read(memcontract.ScopeGlobal, "project_adapter_write.md")
		if err != nil {
			t.Fatalf("Store.Read(adapter write) error = %v", err)
		}
		if !strings.Contains(string(got), "Adapter writes use controller decisions.") {
			t.Fatalf("Store.Read(adapter write) = %q, want applied decision", string(got))
		}
		agentBackend := adapter.ForAgent("ws-alpha", "reviewer", memcontract.AgentTierWorkspace)
		if err := agentBackend.EnsureDirs(); err != nil {
			t.Fatalf("agentBackend.EnsureDirs() error = %v", err)
		}
	})

	t.Run("Should reject missing store operations", func(t *testing.T) {
		t.Parallel()

		adapter := memstore.New(nil)
		if err := adapter.EnsureDirs(); err == nil {
			t.Fatal("Adapter.EnsureDirs(nil store) error = nil, want error")
		}
		if _, _, err := adapter.LoadPromptIndex(memcontract.ScopeGlobal); err == nil {
			t.Fatal("Adapter.LoadPromptIndex(nil store) error = nil, want error")
		}
		if _, err := adapter.List(memcontract.ScopeGlobal); err == nil {
			t.Fatal("Adapter.List(nil store) error = nil, want error")
		}
		if _, err := adapter.Recall(testutil.Context(t), memcontract.Query{}, memcontract.RecallOptions{}); err == nil {
			t.Fatal("Adapter.Recall(nil store) error = nil, want error")
		}
		if err := adapter.ApplyDecision(testutil.Context(t), memcontract.Decision{}); err == nil {
			t.Fatal("Adapter.ApplyDecision(nil store) error = nil, want error")
		}
		agentBackend := adapter.ForAgent("ws-alpha", "reviewer", memcontract.AgentTierWorkspace)
		if err := agentBackend.EnsureDirs(); err == nil {
			t.Fatal("agentBackend.EnsureDirs(nil store) error = nil, want error")
		}
	})

	t.Run("Should reject nil adapter operations", func(t *testing.T) {
		t.Parallel()

		var adapter *memstore.Adapter
		if err := adapter.EnsureDirs(); err == nil {
			t.Fatal("nil Adapter.EnsureDirs() error = nil, want error")
		}
	})
}

func openAdapterTestCatalog(t *testing.T, store *memory.Store) {
	t.Helper()
	if err := store.OpenCatalog(testutil.Context(t)); err != nil {
		t.Fatalf("Store.OpenCatalog() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.CloseCatalog(testutil.Context(t)); err != nil {
			t.Errorf("Store.CloseCatalog() error = %v", err)
		}
	})
}

func adapterPayload(t *testing.T) []byte {
	t.Helper()

	payload, err := yaml.Marshal(map[string]any{
		"name":        "Provider Adapter",
		"description": "Provider adapter recall",
		"type":        memcontract.TypeProject,
	})
	if err != nil {
		t.Fatalf("yaml.Marshal(adapter payload) error = %v", err)
	}
	return []byte("---\n" + string(payload) + "---\nProvider adapter recall should work.\n")
}

func adapterWritePayload(t *testing.T) []byte {
	t.Helper()

	payload, err := yaml.Marshal(map[string]any{
		"name": "Adapter Write",
		"type": memcontract.TypeProject,
	})
	if err != nil {
		t.Fatalf("yaml.Marshal(adapter write payload) error = %v", err)
	}
	return []byte("---\n" + string(payload) + "---\nAdapter writes use controller decisions.\n")
}

func TestAdapterRejectsCanceledContext(t *testing.T) {
	t.Run("Should propagate store context validation", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		adapter := memstore.New(memory.NewStore(
			baseDir+"/agh-home/memory",
			memory.WithCatalogDatabasePath(baseDir+"/agh.db"),
		))
		ctx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		if _, err := adapter.Recall(
			ctx,
			memcontract.Query{QueryText: "adapter"},
			memcontract.RecallOptions{},
		); err == nil {
			t.Fatal("Adapter.Recall(canceled) error = nil, want error")
		}
	})
}
