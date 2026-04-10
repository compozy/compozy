package extensions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/plan"
)

func TestHookDispatchIntegrationAcrossPlanPromptAndAgentPhases(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task_01.md"), []byte(`---
status: pending
title: Demo task
type: backend
complexity: low
---

# Task 01: Demo task
`), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	recordPaths := map[string]string{
		"hook-100": filepath.Join(t.TempDir(), "hook-100.jsonl"),
		"hook-500": filepath.Join(t.TempDir(), "hook-500.jsonl"),
		"hook-900": filepath.Join(t.TempDir(), "hook-900.jsonl"),
	}
	discovered := []DiscoveredExtension{
		discoveredHookChainExtension(
			t,
			binary,
			"hook-100",
			100,
			recordPaths["hook-100"],
			map[string]string{
				"plan.post_discover":       "\nPLAN-100\n",
				"prompt.post_build":        "\nPROMPT-100",
				"agent.pre_session_create": "::AGENT-100",
			},
		),
		discoveredHookChainExtension(
			t,
			binary,
			"hook-500",
			500,
			recordPaths["hook-500"],
			map[string]string{
				"plan.post_discover":       "\nPLAN-500\n",
				"prompt.post_build":        "\nPROMPT-500",
				"agent.pre_session_create": "::AGENT-500",
			},
		),
		discoveredHookChainExtension(
			t,
			binary,
			"hook-900",
			900,
			recordPaths["hook-900"],
			map[string]string{
				"plan.post_discover":       "\nPLAN-900\n",
				"prompt.post_build":        "\nPROMPT-900",
				"agent.pre_session_create": "::AGENT-900",
			},
		),
	}

	restoreDiscovery := stubRunScopeDiscovery(t, DiscoveryResult{Extensions: discovered})
	defer restoreDiscovery()

	cfg := &model.RuntimeConfig{
		WorkspaceRoot: workspaceRoot,
		Name:          "demo",
		TasksDir:      tasksDir,
		Mode:          model.ExecutionModePRDTasks,
		IDE:           model.IDECodex,
		DryRun:        true,
		RunID:         "hooks-integration",
	}
	scope, err := OpenRunScope(context.Background(), cfg, OpenRunScopeOptions{EnableExecutableExtensions: true})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}

	var prep *model.SolvePreparation
	defer func() {
		if prep != nil {
			closeCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := prep.CloseJournal(closeCtx); err != nil {
				t.Fatalf("CloseJournal() error = %v", err)
			}
			return
		}
		closeRunScopeForTest(t, scope)
	}()

	if err := scope.Manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	prep, err = plan.Prepare(context.Background(), cfg, scope)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if len(prep.Jobs) != 1 {
		t.Fatalf("expected one prepared job, got %d", len(prep.Jobs))
	}

	promptText := string(prep.Jobs[0].Prompt)
	assertOrderedSnippets(t, promptText, "PLAN-100", "PLAN-500", "PLAN-900")
	assertOrderedSnippets(t, promptText, "PROMPT-100", "PROMPT-500", "PROMPT-900")

	promptArtifact, err := os.ReadFile(prep.Jobs[0].OutPromptPath)
	if err != nil {
		t.Fatalf("read prompt artifact: %v", err)
	}
	assertOrderedSnippets(t, string(promptArtifact), "PLAN-100", "PLAN-500", "PLAN-900")
	assertOrderedSnippets(t, string(promptArtifact), "PROMPT-100", "PROMPT-500", "PROMPT-900")

	agentPayload := map[string]any{
		"run_id": cfg.RunID,
		"job_id": prep.Jobs[0].SafeName,
		"session_request": map[string]any{
			"prompt": base64.StdEncoding.EncodeToString([]byte("agent-base")),
		},
	}
	mutated, err := scope.Manager.DispatchMutable(context.Background(), HookAgentPreSessionCreate, agentPayload)
	if err != nil {
		t.Fatalf("DispatchMutable(agent.pre_session_create) error = %v", err)
	}

	mutatedPayload, ok := mutated.(map[string]any)
	if !ok {
		t.Fatalf("mutated payload type = %T, want map[string]any", mutated)
	}
	sessionRequest, ok := mutatedPayload["session_request"].(map[string]any)
	if !ok {
		t.Fatalf("session_request type = %T, want map[string]any", mutatedPayload["session_request"])
	}
	encodedPrompt, ok := sessionRequest["prompt"].(string)
	if !ok {
		t.Fatalf("session_request.prompt type = %T, want string", sessionRequest["prompt"])
	}
	decodedPrompt, err := base64.StdEncoding.DecodeString(encodedPrompt)
	if err != nil {
		t.Fatalf("decode mutated prompt: %v", err)
	}
	if got, want := string(decodedPrompt), "agent-base::AGENT-100::AGENT-500::AGENT-900"; got != want {
		t.Fatalf("unexpected mutated prompt\nwant: %q\ngot:  %q", want, got)
	}

	scope.Manager.DispatchObserver(context.Background(), HookAgentPostSessionEnd, map[string]any{
		"run_id":     cfg.RunID,
		"job_id":     prep.Jobs[0].SafeName,
		"session_id": "sess-integration",
		"outcome": map[string]any{
			"status": model.StatusCompleted,
		},
	})
	if err := scope.Manager.waitForObservers(context.Background()); err != nil {
		t.Fatalf("waitForObservers() error = %v", err)
	}

	for name, recordPath := range recordPaths {
		records := waitForRecords(t, recordPath, 4)
		findExecuteHookRecord(t, records, "plan.post_discover")
		findExecuteHookRecord(t, records, "prompt.post_build")
		findExecuteHookRecord(t, records, "agent.pre_session_create")
		findExecuteHookRecord(t, records, "agent.post_session_end")
		if len(records) < 4 {
			t.Fatalf("expected %s to record at least 4 events, got %d", name, len(records))
		}
	}
}

func discoveredHookChainExtension(
	t *testing.T,
	binary string,
	name string,
	priority int,
	recordPath string,
	suffixes map[string]string,
) DiscoveredExtension {
	t.Helper()

	suffixesJSON, err := json.Marshal(suffixes)
	if err != nil {
		t.Fatalf("marshal suffixes for %s: %v", name, err)
	}

	supportedHooks := []string{
		string(HookPlanPostDiscover),
		string(HookPromptPostBuild),
		string(HookAgentPreSessionCreate),
		string(HookAgentPostSessionEnd),
	}

	root := filepath.Join(t.TempDir(), name)
	return DiscoveredExtension{
		Ref: Ref{Name: name, Source: SourceWorkspace},
		Manifest: &Manifest{
			Extension: ExtensionInfo{
				Name:              name,
				Version:           "1.0.0",
				MinCompozyVersion: "0.0.0",
			},
			Subprocess: &SubprocessConfig{
				Command: binary,
				Env: map[string]string{
					"COMPOZY_MOCK_MODE":                 "normal",
					"COMPOZY_MOCK_RECORD_PATH":          recordPath,
					"COMPOZY_MOCK_SUPPORTED_HOOKS":      strings.Join(supportedHooks, ","),
					"COMPOZY_MOCK_APPEND_SUFFIXES_JSON": string(suffixesJSON),
				},
			},
			Security: SecurityConfig{
				Capabilities: []Capability{
					CapabilityPlanMutate,
					CapabilityPromptMutate,
					CapabilityAgentMutate,
				},
			},
			Hooks: []HookDeclaration{
				{Event: HookPlanPostDiscover, Priority: priority},
				{Event: HookPromptPostBuild, Priority: priority},
				{Event: HookAgentPreSessionCreate, Priority: priority},
				{Event: HookAgentPostSessionEnd, Priority: priority},
			},
		},
		ExtensionDir: root,
		ManifestPath: filepath.Join(root, "compozy.toml"),
		Enabled:      true,
	}
}

func assertOrderedSnippets(t *testing.T, text string, snippets ...string) {
	t.Helper()

	lastIndex := -1
	for _, snippet := range snippets {
		index := strings.Index(text, snippet)
		if index < 0 {
			t.Fatalf("expected text to contain %q, got:\n%s", snippet, text)
		}
		if index <= lastIndex {
			t.Fatalf("expected snippet %q to appear after prior snippets in:\n%s", snippet, text)
		}
		lastIndex = index
	}
}

func findExecuteHookRecord(t *testing.T, records []mockRecord, event string) mockRecord {
	t.Helper()

	for _, record := range records {
		if record.Type != "execute_hook" {
			continue
		}
		if got := record.Payload["event"]; got == event {
			return record
		}
	}
	t.Fatalf("missing execute_hook record for %q in %#v", event, records)
	return mockRecord{}
}
