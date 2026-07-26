package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func TestMemoryCommandTreeHardCutsLegacyVerbs(t *testing.T) {
	t.Parallel()

	root := newRootCommand(commandDeps{})
	expectedLeaves := [][]string{
		{"memory", "list"},
		{"memory", "show"},
		{"memory", "write"},
		{"memory", "edit"},
		{"memory", "delete"},
		{"memory", "search"},
		{"memory", "reindex"},
		{"memory", "history"},
		{"memory", "health"},
		{"memory", "promote"},
		{"memory", "reset"},
		{"memory", "reload"},
		{"memory", "scope-show"},
		{"memory", "decisions", "list"},
		{"memory", "decisions", "show"},
		{"memory", "decisions", "revert"},
		{"memory", "recall", "trace"},
		{"memory", "dream", "show"},
		{"memory", "dream", "retry"},
		{"memory", "dream", "trigger"},
		{"memory", "dream", "status"},
		{"memory", "daily", "ls"},
		{"memory", "daily", "show"},
		{"memory", "daily", "archive"},
		{"memory", "daily", "restore"},
		{"memory", "daily", "purge"},
		{"memory", "extractor", "status"},
		{"memory", "extractor", "list-pending"},
		{"memory", "extractor", "replay"},
		{"memory", "extractor", "drain"},
		{"memory", "extractor", "disable"},
		{"memory", "provider", "list"},
		{"memory", "provider", "enable"},
		{"memory", "provider", "disable"},
		{"memory", "adhoc", "list"},
		{"memory", "adhoc", "show"},
	}
	for _, args := range expectedLeaves {
		cmd, remaining, err := root.Find(args)
		if err != nil {
			t.Fatalf("Find(%v) error = %v", args, err)
		}
		if len(remaining) != 0 {
			t.Fatalf("Find(%v) remaining = %v, want none", args, remaining)
		}
		if got := strings.TrimSpace(cmd.CommandPath()); got != "agh "+strings.Join(args, " ") {
			t.Fatalf("CommandPath(%v) = %q", args, got)
		}
	}

	for _, legacy := range [][]string{{"memory", "read"}, {"memory", "consolidate"}} {
		cmd, remaining, err := root.Find(legacy)
		if err == nil && len(remaining) == 0 &&
			strings.TrimSpace(cmd.CommandPath()) == "agh "+strings.Join(legacy, " ") {
			t.Fatalf("legacy command %v resolved to a leaf", legacy)
		}
	}
}

func TestMemoryListShowAndSearchUseV2Selectors(t *testing.T) {
	t.Parallel()

	var seenList MemoryListQuery
	var seenShowSelector MemorySelectorQuery
	var seenSearch MemorySearchRequest
	deps := newTestDeps(t, &stubClient{
		listMemoryFn: func(_ context.Context, query MemoryListQuery) (MemoryListRecord, error) {
			seenList = query
			return MemoryListRecord{Memories: []contract.MemoryEntrySummaryPayload{{
				Filename:    "prefs.md",
				Name:        "Prefs",
				Description: "saved preference",
				Type:        memcontract.TypeUser,
				Scope:       memcontract.ScopeAgent,
				AgentName:   "reviewer",
				AgentTier:   memcontract.AgentTierGlobal,
				ModTime:     fixedTestNow,
				Injection:   true,
			}}}, nil
		},
		showMemoryFn: func(
			_ context.Context,
			filename string,
			query MemorySelectorQuery,
		) (MemoryEntryRecord, error) {
			if filename != "prefs.md" {
				t.Fatalf("ShowMemory filename = %q, want prefs.md", filename)
			}
			seenShowSelector = query
			return MemoryEntryRecord{Memory: contract.MemoryEntryPayload{
				Summary: contract.MemoryEntrySummaryPayload{
					Filename:  "prefs.md",
					Scope:     memcontract.ScopeAgent,
					AgentName: "reviewer",
					AgentTier: memcontract.AgentTierGlobal,
				},
				Content: "stored memory body",
			}}, nil
		},
		searchMemoryFn: func(_ context.Context, request MemorySearchRequest) (MemorySearchRecord, error) {
			seenSearch = request
			return MemorySearchRecord{Results: []contract.MemorySearchResultPayload{{
				Memory: contract.MemoryEntrySummaryPayload{
					Filename:  "prefs.md",
					Name:      "Prefs",
					Scope:     memcontract.ScopeAgent,
					AgentTier: memcontract.AgentTierGlobal,
				},
				Score:   1,
				Snippet: "stored memory body",
			}}}, nil
		},
	})

	listOut, _, err := executeRootCommand(
		t,
		deps,
		"memory",
		"list",
		"--scope",
		"agent",
		"--agent",
		"reviewer",
		"--agent-tier",
		"global",
		"--type",
		"user",
		"--sort",
		"name",
		"--cursor",
		"next-page",
		"--limit",
		"7",
		"--include-system",
		"-o",
		"jsonl",
	)
	if err != nil {
		t.Fatalf("memory list error = %v", err)
	}
	if seenList.Scope != memcontract.ScopeAgent ||
		seenList.AgentName != "reviewer" ||
		seenList.AgentTier != memcontract.AgentTierGlobal ||
		seenList.Type != memcontract.TypeUser ||
		seenList.Sort != "name" ||
		seenList.Cursor != "next-page" ||
		seenList.Limit != 7 ||
		!seenList.IncludeSystem {
		t.Fatalf("list query = %#v, want agent selector with filters", seenList)
	}
	if got := strings.Count(strings.TrimSpace(listOut), "\n") + 1; got != 1 {
		t.Fatalf("list jsonl lines = %d, output=%q", got, listOut)
	}
	if _, _, err := executeRootCommand(t, deps, "memory", "list", "--include-shadowed"); err == nil {
		t.Fatal("memory list --include-shadowed error = nil, want removed flag")
	}

	showOut, _, err := executeRootCommand(
		t,
		deps,
		"memory",
		"show",
		"prefs.md",
		"--scope",
		"agent",
		"--agent",
		"reviewer",
		"--agent-tier",
		"global",
	)
	if err != nil {
		t.Fatalf("memory show error = %v", err)
	}
	if strings.TrimSpace(showOut) != "stored memory body" {
		t.Fatalf("show output = %q, want raw content", showOut)
	}
	if seenShowSelector.Scope != memcontract.ScopeAgent ||
		seenShowSelector.AgentName != "reviewer" ||
		seenShowSelector.AgentTier != memcontract.AgentTierGlobal {
		t.Fatalf("show selector = %#v, want agent selector", seenShowSelector)
	}

	searchOut, _, err := executeRootCommand(
		t,
		deps,
		"memory",
		"search",
		"review",
		"tone",
		"--scope",
		"agent",
		"--agent",
		"reviewer",
		"--agent-tier",
		"global",
		"--top-k",
		"3",
		"--include-system",
	)
	if err != nil {
		t.Fatalf("memory search error = %v", err)
	}
	if seenSearch.QueryText != "review tone" ||
		seenSearch.Scope != memcontract.ScopeAgent ||
		seenSearch.AgentName != "reviewer" ||
		seenSearch.AgentTier != memcontract.AgentTierGlobal ||
		seenSearch.TopK != 3 ||
		!seenSearch.IncludeSystem {
		t.Fatalf("search request = %#v, want agent query", seenSearch)
	}
	if !strings.Contains(searchOut, "prefs.md") {
		t.Fatalf("search output = %q, want result filename", searchOut)
	}
	if !strings.Contains(searchOut, "agent:global") {
		t.Fatalf("search output = %q, want agent tier label", searchOut)
	}
}

func TestMemoryDecisionCommands(t *testing.T) {
	t.Run("Should pass target filename and limit through the decisions list command", func(t *testing.T) {
		t.Parallel()

		var seen MemoryDecisionListQuery
		deps := newTestDeps(t, &stubClient{
			listMemoryDecisionsFn: func(
				_ context.Context,
				query MemoryDecisionListQuery,
			) (MemoryDecisionListRecord, error) {
				seen = query
				return MemoryDecisionListRecord{Decisions: []contract.MemoryDecisionPayload{
					testMemoryDecision("dec-filtered", memcontract.OpUpdate),
				}}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"memory",
			"decisions",
			"list",
			"--scope",
			"global",
			"--filename",
			"prefs.md",
			"--limit",
			"1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("memory decisions list error = %v", err)
		}
		if seen.Scope != memcontract.ScopeGlobal || seen.TargetFilename != "prefs.md" || seen.Limit != 1 {
			t.Fatalf("memory decision query = %#v, want global prefs.md limit 1", seen)
		}
		if !strings.Contains(stdout, "dec-filtered") {
			t.Fatalf("memory decisions list output = %q, want filtered decision", stdout)
		}
	})
}

func TestMemoryExtractorDrainUsesTimeoutContext(t *testing.T) {
	t.Parallel()

	t.Run("Should apply explicit timeout", func(t *testing.T) {
		t.Parallel()

		var remaining time.Duration
		deps := newTestDeps(t, &stubClient{
			drainMemoryExtractorFn: func(ctx context.Context) (MemoryExtractorDrainRecord, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("DrainMemoryExtractor context deadline missing")
				}
				remaining = time.Until(deadline)
				return MemoryExtractorDrainRecord{}, nil
			},
		})

		cmd := newRootCommand(deps)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"memory", "extractor", "drain", "--timeout", "5s"})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("memory extractor drain error = %v", err)
		}
		if remaining < 4*time.Second || remaining > 6*time.Second {
			t.Fatalf("drain timeout remaining = %s, want about 5s", remaining)
		}
	})

	t.Run("Should default timeout to sixty seconds", func(t *testing.T) {
		t.Parallel()

		var remaining time.Duration
		deps := newTestDeps(t, &stubClient{
			drainMemoryExtractorFn: func(ctx context.Context) (MemoryExtractorDrainRecord, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("DrainMemoryExtractor context deadline missing")
				}
				remaining = time.Until(deadline)
				return MemoryExtractorDrainRecord{}, nil
			},
		})

		cmd := newRootCommand(deps)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"memory", "extractor", "drain"})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("memory extractor drain error = %v", err)
		}
		if remaining < 55*time.Second || remaining > 65*time.Second {
			t.Fatalf("drain timeout remaining = %s, want about 60s", remaining)
		}
	})
}

func TestMemoryWriteEditDeleteAndReindexUsePublicPayloads(t *testing.T) {
	t.Parallel()

	contentPath := filepath.Join(t.TempDir(), "memory.md")
	if err := os.WriteFile(contentPath, []byte("remember the runtime contract"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(content) error = %v", err)
	}

	var createRequest MemoryCreateRequest
	var editRequest MemoryEditRequest
	var deleteSelector MemorySelectorQuery
	var reindexRequest MemoryReindexRequest
	deps := newTestDeps(t, &stubClient{
		createMemoryFn: func(_ context.Context, request MemoryCreateRequest) (MemoryMutationRecord, error) {
			createRequest = request
			return MemoryMutationRecord{
				Decision: testMemoryDecision("dec-create", memcontract.OpAdd),
				Applied:  true,
			}, nil
		},
		editMemoryFn: func(_ context.Context, filename string, request MemoryEditRequest) (MemoryMutationRecord, error) {
			if filename != "prefs.md" {
				t.Fatalf("edit filename = %q, want prefs.md", filename)
			}
			editRequest = request
			return MemoryMutationRecord{
				Decision: testMemoryDecision("dec-edit", memcontract.OpUpdate),
				Applied:  true,
			}, nil
		},
		deleteMemoryFn: func(_ context.Context, filename string, query MemorySelectorQuery) (MemoryDeleteRecord, error) {
			if filename != "prefs.md" {
				t.Fatalf("delete filename = %q, want prefs.md", filename)
			}
			deleteSelector = query
			return MemoryDeleteRecord{
				Decision: testMemoryDecision("dec-delete", memcontract.OpDelete),
				Applied:  true,
			}, nil
		},
		reindexMemoryFn: func(_ context.Context, request MemoryReindexRequest) (MemoryReindexRecord, error) {
			reindexRequest = request
			return MemoryReindexRecord{
				IndexedFiles: 2,
				Scope:        memcontract.ScopeWorkspace,
				WorkspaceID:  "/workspace/project",
				CompletedAt:  fixedTestNow,
			}, nil
		},
	})

	writeOut, _, err := executeRootCommand(
		t,
		deps,
		"memory",
		"write",
		"--scope",
		"workspace",
		"--type",
		"project",
		"--name",
		"Runtime Contract",
		"--description",
		"runtime memory",
		"--content",
		"@"+contentPath,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("memory write error = %v", err)
	}
	if createRequest.Scope != memcontract.ScopeWorkspace ||
		createRequest.WorkspaceID != "/workspace/project" ||
		createRequest.Type != memcontract.TypeProject ||
		createRequest.Name != "Runtime Contract" ||
		createRequest.Description != "runtime memory" ||
		createRequest.Content != "remember the runtime contract" ||
		createRequest.Origin != memcontract.OriginCLI {
		t.Fatalf("create request = %#v", createRequest)
	}
	var writePayload MemoryMutationRecord
	if err := json.Unmarshal([]byte(writeOut), &writePayload); err != nil {
		t.Fatalf("json.Unmarshal(write) error = %v; out=%s", err, writeOut)
	}
	if writePayload.Decision.ID != "dec-create" || !writePayload.Applied {
		t.Fatalf("write payload = %#v", writePayload)
	}

	if _, _, err := executeRootCommand(
		t,
		deps,
		"memory",
		"edit",
		"prefs.md",
		"--scope",
		"workspace",
		"--content",
		"updated body",
	); err != nil {
		t.Fatalf("memory edit error = %v", err)
	}
	if editRequest.Scope != memcontract.ScopeWorkspace ||
		editRequest.WorkspaceID != "/workspace/project" ||
		editRequest.Content != "updated body" {
		t.Fatalf("edit request = %#v", editRequest)
	}

	if _, _, err := executeRootCommand(t, deps, "memory", "delete", "prefs.md", "--scope", "workspace"); err != nil {
		t.Fatalf("memory delete error = %v", err)
	}
	if deleteSelector.Scope != memcontract.ScopeWorkspace || deleteSelector.WorkspaceID != "/workspace/project" {
		t.Fatalf("delete selector = %#v", deleteSelector)
	}

	if _, _, err := executeRootCommand(t, deps, "memory", "reindex", "--scope", "workspace"); err != nil {
		t.Fatalf("memory reindex error = %v", err)
	}
	if reindexRequest.Scope != memcontract.ScopeWorkspace || reindexRequest.WorkspaceID != "/workspace/project" {
		t.Fatalf("reindex request = %#v", reindexRequest)
	}
}

func TestMemoryNestedOperationsCallDaemonClient(t *testing.T) {
	t.Parallel()

	calls := make(map[string]bool)
	deps := newTestDeps(t, &stubClient{
		memoryHealthFn: func(_ context.Context, workspace string) (MemoryHealthRecord, error) {
			calls["health"] = workspace == "/workspace/project"
			return MemoryHealthRecord{Status: "ok", Enabled: true, Configured: true}, nil
		},
		memoryHistoryFn: func(_ context.Context, query MemoryHistoryQuery) ([]MemoryHistoryRecord, error) {
			calls["history"] = query.Operation == "memory.write" && query.Limit == 7
			return []MemoryHistoryRecord{
				{ID: "evt-1", Operation: memcontract.OperationWrite, Timestamp: fixedTestNow},
			}, nil
		},
		promoteMemoryFn: func(_ context.Context, request MemoryPromoteRequest) (MemoryPromoteRecord, error) {
			calls["promote"] = request.Filename == "prefs.md" &&
				request.From.Scope == memcontract.ScopeWorkspace &&
				request.To.Scope == memcontract.ScopeAgent &&
				request.To.AgentTier == memcontract.AgentTierGlobal
			return MemoryPromoteRecord{
				Decision: testMemoryDecision("dec-promote", memcontract.OpUpdate),
				Applied:  true,
			}, nil
		},
		memoryScopeShowFn: func(_ context.Context, query MemorySelectorQuery) (MemoryScopeShowRecord, error) {
			calls["scope-show"] = query.Scope == memcontract.ScopeAgent && query.AgentName == "reviewer"
			return MemoryScopeShowRecord{
				Selector: contract.MemoryScopeSelectorPayload{Scope: memcontract.ScopeAgent},
			}, nil
		},
		triggerMemoryDreamFn: func(_ context.Context, request MemoryDreamTriggerRequest) (MemoryDreamTriggerRecord, error) {
			calls["dream-trigger"] = request.Scope == memcontract.ScopeWorkspace &&
				request.WorkspaceID == "/workspace/project"
			return MemoryDreamTriggerRecord{Triggered: true, Dream: contract.MemoryDreamPayload{
				Status:    contract.MemoryDreamStateRunning,
				Scope:     memcontract.ScopeWorkspace,
				StartedAt: fixedTestNow,
			}}, nil
		},
		listMemoryProvidersFn: func(context.Context) (MemoryProviderListRecord, error) {
			calls["provider-list"] = true
			return MemoryProviderListRecord{
				Providers: []contract.MemoryProviderPayload{{Name: "local", Active: true}},
			}, nil
		},
		enableMemoryProviderFn: func(_ context.Context, name string, _ MemoryProviderLifecycleRequest) (MemoryProviderLifecycleRecord, error) {
			calls["provider-enable"] = name == "local"
			return MemoryProviderLifecycleRecord{
				Provider: contract.MemoryProviderPayload{Name: name, Active: true},
				Changed:  true,
			}, nil
		},
		getMemoryExtractorStatusFn: func(_ context.Context, sessionID string) (MemoryExtractorStatusRecord, error) {
			calls["extractor-status"] = sessionID == "sess-1"
			return MemoryExtractorStatusRecord{
				Extractor: contract.MemoryExtractorStatusPayload{Status: contract.MemoryExtractorStateStopped},
			}, nil
		},
		listMemoryDailyLogsFn: func(_ context.Context, query MemorySelectorQuery) (MemoryDailyLogListRecord, error) {
			calls["daily-ls"] = query.Scope == memcontract.ScopeWorkspace
			return MemoryDailyLogListRecord{
				Logs: []contract.MemoryDailyLogPayload{{Date: "2026-05-05", Scope: memcontract.ScopeWorkspace}},
			}, nil
		},
	})

	commands := [][]string{
		{"memory", "health", "-o", "json"},
		{"memory", "history", "--operation", "memory.write", "--limit", "7", "-o", "json"},
		{
			"memory", "promote", "prefs.md",
			"--from", "workspace",
			"--to", "agent:global",
			"--agent", "reviewer",
			"-o", "json",
		},
		{
			"memory", "scope-show",
			"--scope", "agent",
			"--agent", "reviewer",
			"--agent-tier", "global",
			"-o", "json",
		},
		{"memory", "dream", "trigger", "--scope", "workspace", "-o", "json"},
		{"memory", "provider", "list", "-o", "json"},
		{"memory", "provider", "enable", "local", "-o", "json"},
		{"memory", "extractor", "status", "--session", "sess-1", "-o", "json"},
		{"memory", "daily", "ls", "--scope", "workspace", "-o", "json"},
	}
	for _, args := range commands {
		if _, _, err := executeRootCommand(t, deps, args...); err != nil {
			t.Fatalf("executeRootCommand(%v) error = %v", args, err)
		}
	}
	for _, name := range []string{
		"health",
		"history",
		"promote",
		"scope-show",
		"dream-trigger",
		"provider-list",
		"provider-enable",
		"extractor-status",
		"daily-ls",
	} {
		if !calls[name] {
			t.Fatalf("expected call %q", name)
		}
	}
}

func TestMemorySelectorValidationAndUnsupportedCommands(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{})
	if _, _, err := executeRootCommand(
		t,
		deps,
		"memory",
		"list",
		"--scope",
		"agent",
		"--agent",
		"reviewer",
	); err == nil ||
		!strings.Contains(err.Error(), "memory.scope.agent_tier_required") {
		t.Fatalf("agent tier validation error = %v", err)
	}
	if _, _, err := executeRootCommand(t, deps, "memory", "list", "--scope", "bogus"); err == nil ||
		!strings.Contains(err.Error(), "memory.scope.invalid") {
		t.Fatalf("invalid scope error = %v", err)
	}
	if _, _, err := executeRootCommand(
		t,
		deps,
		"memory",
		"daily",
		"archive",
		"--older-than",
		"7d",
		"--dry-run",
	); err == nil ||
		!strings.Contains(err.Error(), "memory.unsupported") {
		t.Fatalf("daily archive unsupported error = %v", err)
	}
}

func TestMemoryBundleHelpers(t *testing.T) {
	t.Parallel()

	listBundle := memoryListBundle(MemoryListRecord{Memories: []contract.MemoryEntrySummaryPayload{{
		Filename:    "prefs.md",
		Name:        "Prefs",
		Type:        memcontract.TypeUser,
		Description: "saved preference",
		Scope:       memcontract.ScopeAgent,
		AgentTier:   memcontract.AgentTierGlobal,
		ModTime:     fixedTestNow.Add(-time.Minute),
	}}}, func() time.Time { return fixedTestNow })
	listHuman, err := listBundle.human()
	if err != nil {
		t.Fatalf("listBundle.human() error = %v", err)
	}
	if !strings.Contains(listHuman, "agent:global") {
		t.Fatalf("list human = %q, want agent tier label", listHuman)
	}

	decisionBundle := memoryMutationBundle("Memory Write", MemoryMutationRecord{
		Decision: testMemoryDecision("dec-1", memcontract.OpAdd),
		Applied:  true,
	})
	decisionHuman, err := decisionBundle.human()
	if err != nil {
		t.Fatalf("decisionBundle.human() error = %v", err)
	}
	if !strings.Contains(decisionHuman, "dec-1") || !strings.Contains(decisionHuman, "add") {
		t.Fatalf("decision human = %q", decisionHuman)
	}

	if memoryScopeLabel(memcontract.ScopeAgent, memcontract.AgentTierWorkspace) != "agent:workspace" {
		t.Fatalf(
			"memoryScopeLabel(agent workspace) = %q",
			memoryScopeLabel(memcontract.ScopeAgent, memcontract.AgentTierWorkspace),
		)
	}
	if boolStatus(false) != "false" {
		t.Fatalf("boolStatus(false) = %q, want false", boolStatus(false))
	}
	if _, err := parseOptionalCLIMemoryScope("bogus"); err == nil {
		t.Fatal("parseOptionalCLIMemoryScope(bogus) error = nil, want non-nil")
	}
	if _, err := parseOptionalCLIAgentTier("bogus"); err == nil {
		t.Fatal("parseOptionalCLIAgentTier(bogus) error = nil, want non-nil")
	}
	if _, err := parseOptionalMemoryType("bogus"); err == nil {
		t.Fatal("parseOptionalMemoryType(bogus) error = nil, want non-nil")
	}
	if _, err := resolveMemoryContentValue(newTestDeps(t, &stubClient{}), "@", strings.NewReader("")); err == nil {
		t.Fatal("resolveMemoryContentValue(@) error = nil, want non-nil")
	}
	if _, err := resolveMemoryContentValue(newTestDeps(t, &stubClient{}), "-", strings.NewReader("")); err == nil {
		t.Fatal("resolveMemoryContentValue(empty stdin) error = nil, want non-nil")
	}
	if _, err := readOptionalCommandInput(nil); err != nil {
		t.Fatalf("readOptionalCommandInput(nil) error = %v", err)
	}
}

func testMemoryDecision(id string, op memcontract.Op) contract.MemoryDecisionPayload {
	return contract.MemoryDecisionPayload{
		ID:             id,
		CandidateHash:  "sha256:test",
		Op:             contract.MemoryDecisionOp(op.String()),
		Scope:          memcontract.ScopeWorkspace,
		TargetFilename: "prefs.md",
		Frontmatter: memcontract.Header{
			Name: "Prefs",
			Type: memcontract.TypeUser,
		},
		Confidence: 0.9,
		Source:     memcontract.SourceRule,
		Reason:     "accepted",
		DecidedAt:  fixedTestNow,
	}
}

func TestMemoryErrorsWrapAsExpected(t *testing.T) {
	t.Parallel()

	err := errors.New("memory.unsupported: reserved")
	if !strings.Contains(err.Error(), "memory.unsupported") {
		t.Fatalf("error = %v", err)
	}
}

func TestMemoryCommandsRejectRemovedLegacyFlags(t *testing.T) {
	t.Parallel()

	deps := commandDeps{}

	t.Run("Should reject reset include-system flag", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(t, deps, "memory", "reset", "--include-system")
		if err == nil {
			t.Fatal("executeRootCommand(memory reset --include-system) error = nil, want unknown flag")
		}
		if !strings.Contains(err.Error(), "unknown flag: --include-system") {
			t.Fatalf("memory reset unknown flag error = %v", err)
		}
	})

	t.Run("Should reject extractor replay from-dlq flag", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(
			t,
			deps,
			"memory",
			"extractor",
			"replay",
			"--session",
			"sess-1",
			"--from-dlq",
		)
		if err == nil {
			t.Fatal("executeRootCommand(memory extractor replay --from-dlq) error = nil, want unknown flag")
		}
		if !strings.Contains(err.Error(), "unknown flag: --from-dlq") {
			t.Fatalf("memory extractor replay unknown flag error = %v", err)
		}
	})
}
