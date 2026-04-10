package extensions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/plan"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/reviews"
	runcore "github.com/compozy/compozy/internal/core/run"
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

func TestHookDispatchIntegrationAcrossRunAndJobPhases(t *testing.T) {
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
		"hook-100": filepath.Join(t.TempDir(), "run-job-hook-100.jsonl"),
		"hook-500": filepath.Join(t.TempDir(), "run-job-hook-500.jsonl"),
		"hook-900": filepath.Join(t.TempDir(), "run-job-hook-900.jsonl"),
	}
	hooks := []HookName{
		HookRunPreStart,
		HookRunPostStart,
		HookJobPreExecute,
		HookJobPostExecute,
		HookRunPreShutdown,
		HookRunPostShutdown,
	}
	discovered := []DiscoveredExtension{
		discoveredRecordedExtension(
			t,
			binary,
			"hook-100",
			100,
			recordPaths["hook-100"],
			[]Capability{CapabilityRunMutate, CapabilityJobMutate},
			hooks,
			"",
			nil,
		),
		discoveredRecordedExtension(
			t,
			binary,
			"hook-500",
			500,
			recordPaths["hook-500"],
			[]Capability{CapabilityRunMutate, CapabilityJobMutate},
			hooks,
			"",
			nil,
		),
		discoveredRecordedExtension(
			t,
			binary,
			"hook-900",
			900,
			recordPaths["hook-900"],
			[]Capability{CapabilityRunMutate, CapabilityJobMutate},
			hooks,
			"",
			nil,
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
		RunID:         "run-job-hooks",
	}
	scope, err := OpenRunScope(context.Background(), cfg, OpenRunScopeOptions{EnableExecutableExtensions: true})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}

	var prep *model.SolvePreparation
	defer func() {
		if prep != nil {
			if err := prep.CloseJournal(context.Background()); err != nil {
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

	if err := runcore.Execute(
		context.Background(),
		prep.Jobs,
		prep.RunArtifacts,
		prep.Journal(),
		prep.EventBus(),
		cfg,
		scope.Manager,
	); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if err := scope.Manager.waitForObservers(context.Background()); err != nil {
		t.Fatalf("waitForObservers() error = %v", err)
	}

	for name, recordPath := range recordPaths {
		records := waitForRecords(t, recordPath, 8)
		gotEvents := executeHookSequence(records)
		assertHookPartialOrder(t, name, gotEvents, [][2]string{
			{"run.pre_start", "run.post_start"},
			{"run.post_start", "job.pre_execute"},
			{"job.pre_execute", "job.post_execute"},
			{"job.pre_execute", "run.pre_shutdown"},
			{"run.pre_shutdown", "run.post_shutdown"},
			{"job.post_execute", "run.post_shutdown"},
		})

		postShutdown := findExecuteHookRecord(t, records, "run.post_shutdown")
		payload := recordPayload(t, postShutdown)
		summary, ok := payload["summary"].(map[string]any)
		if !ok {
			t.Fatalf("run.post_shutdown summary type = %T, want map[string]any", payload["summary"])
		}
		if got := summary["status"]; got != "succeeded" {
			t.Fatalf("run.post_shutdown summary.status = %#v, want %q", got, "succeeded")
		}
	}
}

func TestHookDispatchIntegrationAcrossReviewPhases(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	workspaceRoot := t.TempDir()
	reviewDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo", "reviews-007")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider: "coderabbit",
		PR:       "259",
		Round:    7,
	}, []provider.ReviewItem{
		{
			Title:       "Add nil check",
			File:        "internal/app/service.go",
			Line:        42,
			Author:      "coderabbitai[bot]",
			ProviderRef: "thread:PRT_1,comment:RC_1",
			Body:        "Please add a nil check before dereferencing the pointer.",
		},
	}); err != nil {
		t.Fatalf("write round: %v", err)
	}
	installHookACPHelperOnPath(t, hookACPHelperScenario{
		SessionID: "sess-review-hooks",
		Updates: []acp.SessionUpdate{
			acp.UpdateAgentMessageText("review fixes applied"),
		},
	})

	recordPath := filepath.Join(t.TempDir(), "review-hooks.jsonl")
	restoreDiscovery := stubRunScopeDiscovery(t, DiscoveryResult{Extensions: []DiscoveredExtension{
		discoveredRecordedExtension(
			t,
			binary,
			"review-hooks",
			100,
			recordPath,
			[]Capability{CapabilityReviewMutate},
			[]HookName{
				HookReviewPreFetch,
				HookReviewPostFetch,
				HookReviewPreBatch,
				HookReviewPostFix,
				HookReviewPreResolve,
			},
			`{"resolve":false}`,
			nil,
		),
	}})
	defer restoreDiscovery()

	cfg := &model.RuntimeConfig{
		WorkspaceRoot:          workspaceRoot,
		ReviewsDir:             reviewDir,
		Mode:                   model.ExecutionModePRReview,
		IDE:                    model.IDECodex,
		BatchSize:              10,
		OutputFormat:           model.OutputFormatRawJSON,
		ReasoningEffort:        "medium",
		RetryBackoffMultiplier: 2,
		RunID:                  "review-hooks",
	}
	scope, err := OpenRunScope(context.Background(), cfg, OpenRunScopeOptions{EnableExecutableExtensions: true})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}

	var prep *model.SolvePreparation
	defer func() {
		if prep != nil {
			if err := prep.CloseJournal(context.Background()); err != nil {
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
		t.Fatalf("expected one prepared review job, got %d", len(prep.Jobs))
	}
	issuePath := filepath.Join(reviewDir, "issue_001.md")
	content, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("read issue file: %v", err)
	}
	triaged := strings.Replace(string(content), "status: pending", "status: valid", 1)
	if err := os.WriteFile(issuePath, []byte(triaged), 0o600); err != nil {
		t.Fatalf("write triaged issue file: %v", err)
	}

	if err := runcore.Execute(
		context.Background(),
		prep.Jobs,
		prep.RunArtifacts,
		prep.Journal(),
		prep.EventBus(),
		cfg,
		scope.Manager,
	); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if err := scope.Manager.waitForObservers(context.Background()); err != nil {
		t.Fatalf("waitForObservers() error = %v", err)
	}

	records := waitForRecords(t, recordPath, 7)
	gotEvents := executeHookSequence(records)
	wantPrefix := []string{"review.pre_fetch", "review.post_fetch", "review.pre_batch"}
	if len(gotEvents) < len(wantPrefix) || !reflect.DeepEqual(gotEvents[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("review hook prefix mismatch\nwant: %#v\ngot:  %#v", wantPrefix, gotEvents)
	}
	for _, want := range []string{"review.post_fix", "review.pre_resolve"} {
		if !containsString(gotEvents, want) {
			t.Fatalf("missing review hook %q in %#v", want, gotEvents)
		}
	}

	preFetchPayload := recordPayload(t, findExecuteHookRecord(t, records, "review.pre_fetch"))
	fetchConfig, ok := preFetchPayload["fetch_config"].(map[string]any)
	if !ok {
		t.Fatalf("review.pre_fetch fetch_config type = %T, want map[string]any", preFetchPayload["fetch_config"])
	}
	if got := fetchConfig["reviews_dir"]; got != reviewDir {
		t.Fatalf("review.pre_fetch reviews_dir = %#v, want %q", got, reviewDir)
	}

	postFetchPayload := recordPayload(t, findExecuteHookRecord(t, records, "review.post_fetch"))
	issues, ok := postFetchPayload["issues"].([]any)
	if !ok || len(issues) != 1 {
		t.Fatalf("review.post_fetch issues = %#v, want one issue", postFetchPayload["issues"])
	}

	preBatchPayload := recordPayload(t, findExecuteHookRecord(t, records, "review.pre_batch"))
	groups, ok := preBatchPayload["groups"].(map[string]any)
	if !ok {
		t.Fatalf("review.pre_batch groups type = %T, want map[string]any", preBatchPayload["groups"])
	}
	if _, ok := groups["internal/app/service.go"]; !ok {
		t.Fatalf("review.pre_batch groups missing service file: %#v", groups)
	}

	postFixPayload := recordPayload(t, findExecuteHookRecord(t, records, "review.post_fix"))
	postFixIssue, ok := postFixPayload["issue"].(map[string]any)
	if !ok {
		t.Fatalf("review.post_fix issue type = %T, want map[string]any", postFixPayload["issue"])
	}
	if got := postFixIssue["Name"]; got != "issue_001.md" {
		t.Fatalf("review.post_fix issue.Name = %#v, want %q", got, "issue_001.md")
	}
	postFixOutcome, ok := postFixPayload["outcome"].(map[string]any)
	if !ok {
		t.Fatalf("review.post_fix outcome type = %T, want map[string]any", postFixPayload["outcome"])
	}
	if got := postFixOutcome["status"]; got != "succeeded" {
		t.Fatalf("review.post_fix outcome.status = %#v, want %q", got, "succeeded")
	}

	preResolvePayload := recordPayload(t, findExecuteHookRecord(t, records, "review.pre_resolve"))
	preResolveIssue, ok := preResolvePayload["issue"].(map[string]any)
	if !ok {
		t.Fatalf("review.pre_resolve issue type = %T, want map[string]any", preResolvePayload["issue"])
	}
	if got := preResolveIssue["Name"]; got != "issue_001.md" {
		t.Fatalf("review.pre_resolve issue.Name = %#v, want %q", got, "issue_001.md")
	}
}

func TestHookDispatchIntegrationAcrossArtifactWritePhases(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	recordPath := filepath.Join(t.TempDir(), "artifact-hooks.jsonl")
	restoreDiscovery := stubRunScopeDiscovery(t, DiscoveryResult{Extensions: []DiscoveredExtension{
		discoveredRecordedExtension(
			t,
			binary,
			"artifact-hooks",
			100,
			recordPath,
			[]Capability{CapabilityArtifactsWrite},
			[]HookName{HookArtifactPreWrite, HookArtifactPostWrite},
			"",
			nil,
		),
	}})
	defer restoreDiscovery()

	cfg := &model.RuntimeConfig{
		WorkspaceRoot: t.TempDir(),
		Mode:          model.ExecutionModeExec,
		IDE:           model.IDECodex,
		RunID:         "artifact-hooks",
	}
	scope, err := OpenRunScope(context.Background(), cfg, OpenRunScopeOptions{EnableExecutableExtensions: true})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}
	defer closeRunScopeForTest(t, scope)

	if err := scope.Manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	rt := newHostRuntimeWithRunIDAndManager(
		t,
		[]Capability{CapabilityArtifactsWrite},
		nil,
		"",
		scope.Artifacts.RunID,
		scope.Manager,
	)

	result, err := rt.router.Handle(
		context.Background(),
		"ext",
		"host.artifacts.write",
		mustJSON(t, ArtifactWriteRequest{
			Path:    ".compozy/artifacts/note.txt",
			Content: []byte("hello hook integration"),
		}),
	)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if _, ok := result.(*ArtifactWriteResult); !ok {
		t.Fatalf("result type = %T, want *ArtifactWriteResult", result)
	}
	if err := scope.Manager.waitForObservers(context.Background()); err != nil {
		t.Fatalf("waitForObservers() error = %v", err)
	}

	records := waitForRecords(t, recordPath, 4)
	assertExecuteHookSequence(t, "artifact-hooks", records, []string{
		"artifact.pre_write",
		"artifact.post_write",
	})

	preWritePayload := recordPayload(t, findExecuteHookRecord(t, records, "artifact.pre_write"))
	if got := preWritePayload["path"]; got != ".compozy/artifacts/note.txt" {
		t.Fatalf("artifact.pre_write path = %#v, want %q", got, ".compozy/artifacts/note.txt")
	}

	postWritePayload := recordPayload(t, findExecuteHookRecord(t, records, "artifact.post_write"))
	if got := postWritePayload["bytes_written"]; got != float64(len("hello hook integration")) {
		t.Fatalf("artifact.post_write bytes_written = %#v, want %d", got, len("hello hook integration"))
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

	return discoveredRecordedExtension(
		t,
		binary,
		name,
		priority,
		recordPath,
		[]Capability{
			CapabilityPlanMutate,
			CapabilityPromptMutate,
			CapabilityAgentMutate,
		},
		[]HookName{
			HookPlanPostDiscover,
			HookPromptPostBuild,
			HookAgentPreSessionCreate,
			HookAgentPostSessionEnd,
		},
		"",
		suffixes,
	)
}

func discoveredRecordedExtension(
	t *testing.T,
	binary string,
	name string,
	priority int,
	recordPath string,
	capabilities []Capability,
	hooks []HookName,
	patchJSON string,
	suffixes map[string]string,
) DiscoveredExtension {
	t.Helper()

	env := map[string]string{
		"COMPOZY_MOCK_MODE":            "normal",
		"COMPOZY_MOCK_RECORD_PATH":     recordPath,
		"COMPOZY_MOCK_SUPPORTED_HOOKS": joinHookNames(hooks),
	}
	if strings.TrimSpace(patchJSON) != "" {
		env["COMPOZY_MOCK_PATCH_JSON"] = patchJSON
	}
	if len(suffixes) > 0 {
		suffixesJSON, err := json.Marshal(suffixes)
		if err != nil {
			t.Fatalf("marshal suffixes for %s: %v", name, err)
		}
		env["COMPOZY_MOCK_APPEND_SUFFIXES_JSON"] = string(suffixesJSON)
	}

	hookDecls := make([]HookDeclaration, 0, len(hooks))
	for _, hook := range hooks {
		hookDecls = append(hookDecls, HookDeclaration{Event: hook, Priority: priority})
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
				Env:     env,
			},
			Security: SecurityConfig{Capabilities: capabilities},
			Hooks:    hookDecls,
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

func assertExecuteHookSequence(t *testing.T, name string, records []mockRecord, want []string) {
	t.Helper()

	got := executeHookSequence(records)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s hook sequence mismatch\nwant: %#v\ngot:  %#v", name, want, got)
	}
}

func assertHookPartialOrder(t *testing.T, name string, events []string, pairs [][2]string) {
	t.Helper()

	for _, pair := range pairs {
		firstIdx := indexOfString(events, pair[0])
		secondIdx := indexOfString(events, pair[1])
		if firstIdx < 0 || secondIdx < 0 {
			t.Fatalf("%s missing hook(s) for order check %q -> %q in %#v", name, pair[0], pair[1], events)
		}
		if firstIdx >= secondIdx {
			t.Fatalf("%s hook order mismatch: expected %q before %q in %#v", name, pair[0], pair[1], events)
		}
	}
}

func executeHookSequence(records []mockRecord) []string {
	events := make([]string, 0)
	for _, record := range records {
		if record.Type != "execute_hook" {
			continue
		}
		if event, ok := record.Payload["event"].(string); ok {
			events = append(events, event)
		}
	}
	return events
}

func recordPayload(t *testing.T, record mockRecord) map[string]any {
	t.Helper()

	payload, ok := record.Payload["payload"].(map[string]any)
	if !ok {
		t.Fatalf("record payload type = %T, want map[string]any", record.Payload["payload"])
	}
	return payload
}

func joinHookNames(hooks []HookName) string {
	parts := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		parts = append(parts, string(hook))
	}
	return strings.Join(parts, ",")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func indexOfString(values []string, target string) int {
	for idx, value := range values {
		if value == target {
			return idx
		}
	}
	return -1
}

func TestRunACPHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_HOOK_ACP_HELPER_PROCESS") != "1" {
		return
	}

	var scenario hookACPHelperScenario
	if err := json.Unmarshal([]byte(os.Getenv("GO_HOOK_ACP_HELPER_SCENARIO")), &scenario); err != nil {
		fmt.Fprintf(os.Stderr, "load helper scenario: %v\n", err)
		os.Exit(2)
	}

	agent := &hookACPHelperAgent{
		scenario:  scenario,
		sessionID: firstNonEmpty(scenario.SessionID, "sess-hook-run-1"),
	}
	conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.conn = conn

	<-conn.Done()
	os.Exit(0)
}

type hookACPHelperScenario struct {
	SessionID              string              `json:"session_id,omitempty"`
	ExpectedPromptContains string              `json:"expected_prompt_contains,omitempty"`
	Updates                []acp.SessionUpdate `json:"updates,omitempty"`
}

type hookACPHelperAgent struct {
	conn      *acp.AgentSideConnection
	scenario  hookACPHelperScenario
	sessionID string
}

func (a *hookACPHelperAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersionNumber}, nil
}

func (a *hookACPHelperAgent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: acp.SessionId(a.sessionID)}, nil
}

func (*hookACPHelperAgent) LoadSession(context.Context, acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	return acp.LoadSessionResponse{}, nil
}

func (*hookACPHelperAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *hookACPHelperAgent) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	if want := strings.TrimSpace(a.scenario.ExpectedPromptContains); want != "" {
		if got := helperPromptText(req.Prompt); !strings.Contains(got, want) {
			return acp.PromptResponse{}, &acp.RequestError{
				Code:    4000,
				Message: fmt.Sprintf("prompt %q missing %q", got, want),
			}
		}
	}

	for _, update := range a.scenario.Updates {
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: acp.SessionId(a.sessionID),
			Update:    update,
		}); err != nil {
			return acp.PromptResponse{}, err
		}
	}

	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (*hookACPHelperAgent) Cancel(context.Context, acp.CancelNotification) error {
	return nil
}

func (*hookACPHelperAgent) SetSessionMode(
	context.Context,
	acp.SetSessionModeRequest,
) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

func installHookACPHelperOnPath(t *testing.T, scenario hookACPHelperScenario) {
	t.Helper()

	payload, err := json.Marshal(scenario)
	if err != nil {
		t.Fatalf("marshal helper scenario: %v", err)
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "codex-acp")
	script := fmt.Sprintf("#!/bin/sh\nexec %q -test.run=TestRunACPHelperProcess -- \"$@\"\n", os.Args[0])
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write helper script: %v", err)
	}

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GO_WANT_HOOK_ACP_HELPER_PROCESS", "1")
	t.Setenv("GO_HOOK_ACP_HELPER_SCENARIO", string(payload))
}

func helperPromptText(blocks []acp.ContentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Text != nil {
			parts = append(parts, block.Text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
