//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	reviewAndFixLoopName    = "review-and-fix"
	reviewAndFixTaskName    = "review-e2e"
	reviewInvalidTaskName   = "review-invalid"
	reviewInvalidAgentName  = "reviewer-invalid"
	reviewArtifactWriterID  = toolspkg.ToolID("ext__dev_cycle__write_review_artifacts")
	reviewExpectedTimestamp = "2026-07-26T12:00:00Z"
)

func TestDaemonE2EReviewAndFixShouldRemediateAgentAuthoredArtifacts(t *testing.T) {
	t.Parallel()

	driverPath := acpmock.RequireDriver(t)
	homePaths := e2etest.NewHomePaths(t)
	fixturePath := mockFixturePath(t, "review_and_fix_fixture.json")
	primaryRoot := filepath.Join(homePaths.HomeDir, "review-primary")
	seedReviewAndFixWorkspace(t, primaryRoot, homePaths, driverPath, fixturePath)

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		HomePaths: homePaths,
		Workspace: e2etest.WorkspaceSeedOptions{Root: primaryRoot},
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultProvider: acpmock.ProviderName,
			PermissionMode:  config.PermissionModeApproveAll,
			Providers: map[string]config.ProviderConfig{
				acpmock.ProviderName: acpmock.ProviderConfig(driverPath),
			},
		},
		StartTimeout: 30 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	waitForLoopCatalogEntry(t, ctx, harness, reviewAndFixLoopName)
	primaryWorkspaceID := harness.WorkspaceID

	t.Run("Should reject traversal and escaping task symlinks at the daemon extension boundary", func(t *testing.T) {
		assertReviewWriterContainment(t, ctx, harness, primaryWorkspaceID, primaryRoot)
	})

	var primaryBeforeSecond [][]byte
	t.Run("Should review write fix finalize and stop after a clean second generation", func(t *testing.T) {
		started := runReviewAndFixCLI(t, ctx, harness, primaryRoot, reviewAndFixTaskName, "")
		detail := waitForReviewLoopStatus(
			t,
			ctx,
			harness,
			primaryWorkspaceID,
			started.ID,
			contract.LoopRunStatusDone,
		)
		assertReviewAndFixRunContract(t, detail)
		primaryBeforeSecond = assertReviewAndFixArtifacts(t, primaryRoot)
	})

	t.Run("Should keep identical task names isolated in a second workspace", func(t *testing.T) {
		secondaryRoot := filepath.Join(homePaths.HomeDir, "review-secondary")
		seedReviewAndFixWorkspace(t, secondaryRoot, homePaths, driverPath, fixturePath)
		secondary, err := harness.ResolveWorkspace(ctx, secondaryRoot)
		if err != nil {
			t.Fatalf("ResolveWorkspace(secondary) error = %v", err)
		}
		started := runReviewAndFixCLI(t, ctx, harness, secondaryRoot, reviewAndFixTaskName, "")
		detail := waitForReviewLoopStatus(
			t,
			ctx,
			harness,
			secondary.ID,
			started.ID,
			contract.LoopRunStatusDone,
		)
		assertReviewAndFixRunContract(t, detail)
		assertReviewAndFixArtifacts(t, secondaryRoot)
		assertReviewArtifactsUnchanged(t, primaryRoot, primaryBeforeSecond)
		if _, err := os.Stat(reviewRoundPath(primaryRoot, reviewAndFixTaskName, 2)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("primary reviews-002 error = %v, want no cross-workspace round", err)
		}
	})

	t.Run("Should fail schema-invalid reviewer output without a partial round", func(t *testing.T) {
		started := runReviewAndFixCLI(
			t,
			ctx,
			harness,
			primaryRoot,
			reviewInvalidTaskName,
			reviewInvalidAgentName,
		)
		detail := waitForReviewLoopTerminal(t, ctx, harness, primaryWorkspaceID, started.ID)
		if detail.Run.Status == contract.LoopRunStatusDone {
			t.Fatalf("schema-invalid review status = %q, want failure", detail.Run.Status)
		}
		review := requireReviewGenerationOutput(t, detail, 1, "review")
		if review.Status != "failed" || !strings.Contains(review.OutputRef, "action_schema_invalid") {
			t.Fatalf("schema-invalid review output = %#v, want structured schema failure", review)
		}
		if _, err := os.Stat(reviewRoundPath(primaryRoot, reviewInvalidTaskName, 1)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("schema-invalid review round error = %v, want no partial round", err)
		}
	})
}

func seedReviewAndFixWorkspace(
	t testing.TB,
	root string,
	homePaths config.HomePaths,
	driverPath string,
	fixturePath string,
) {
	t.Helper()
	for _, taskName := range []string{reviewAndFixTaskName, reviewInvalidTaskName} {
		taskDir := filepath.Join(root, ".compozy", "tasks", taskName)
		if err := os.MkdirAll(taskDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", taskDir, err)
		}
		content := "---\nstatus: in_progress\ntitle: Review fixture\n---\n\nReview this task.\n"
		if err := os.WriteFile(filepath.Join(taskDir, "task_01.md"), []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%s task) error = %v", taskName, err)
		}
	}

	workspacePaths := homePaths
	workspacePaths.AgentsDir = filepath.Join(root, config.DirName, config.AgentsDirName)
	for _, agent := range []struct {
		name    string
		fixture string
	}{
		{name: "reviewer", fixture: "review_and_fix_reviewer"},
		{name: "review_fixer", fixture: "review_and_fix_fixer"},
		{name: reviewInvalidAgentName, fixture: "review_and_fix_invalid_reviewer"},
	} {
		e2etest.WriteAgentDef(t, workspacePaths, e2etest.AgentSeed{
			Name:        agent.name,
			Provider:    acpmock.ProviderName,
			Command:     acpmock.BuildCommand(driverPath, fixturePath, agent.fixture, ""),
			Permissions: string(config.PermissionModeApproveAll),
			Prompt:      "Deterministic review-and-fix ACP fixture.",
		})
	}
}

func runReviewAndFixCLI(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceRoot string,
	taskName string,
	reviewer string,
) contract.LoopRunPayload {
	t.Helper()
	args := []string{
		"loop", "run",
		"--workspace", workspaceRoot,
		"--name", reviewAndFixLoopName,
		"--input", "task_name=" + taskName,
	}
	if strings.TrimSpace(reviewer) != "" {
		args = append(args, "--input", "reviewer="+strings.TrimSpace(reviewer))
	}
	args = append(args, "-o", "json")
	var response contract.RunLoopResponse
	if err := harness.CLI.RunJSONInDir(ctx, workspaceRoot, &response, args...); err != nil {
		t.Fatalf("CLI review-and-fix run error = %v", err)
	}
	if response.Run == nil {
		t.Fatalf("CLI review-and-fix response = %#v, want persisted run", response)
	}
	return *response.Run
}

func waitForReviewLoopStatus(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	runID string,
	want contract.LoopRunStatus,
) contract.LoopRunResponse {
	t.Helper()
	return waitForReviewLoop(t, ctx, harness, workspaceID, runID, func(status contract.LoopRunStatus) bool {
		return status == want
	}, want)
}

func waitForReviewLoopTerminal(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	runID string,
) contract.LoopRunResponse {
	t.Helper()
	return waitForReviewLoop(t, ctx, harness, workspaceID, runID, loopRunStatusTerminal, "terminal")
}

func waitForReviewLoop(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	runID string,
	done func(contract.LoopRunStatus) bool,
	want any,
) contract.LoopRunResponse {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	path := "/api/workspaces/" + url.PathEscape(workspaceID) + "/loop-runs/" + url.PathEscape(runID)
	var last contract.LoopRunResponse
	for {
		if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &last); err == nil {
			if done(last.Run.Status) {
				return last
			}
			if loopRunStatusTerminal(last.Run.Status) {
				t.Fatalf("wait review loop %s reached unexpected terminal status %q; last=%#v", want, last.Run.Status, last)
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait review loop %s: %v; last=%#v", want, ctx.Err(), last)
		case <-ticker.C:
		}
	}
}

func assertReviewAndFixRunContract(t testing.TB, detail contract.LoopRunResponse) {
	t.Helper()
	if detail.Run.Status != contract.LoopRunStatusDone || detail.Run.Generation != 2 {
		t.Fatalf("review-and-fix run = %#v, want done in generation 2", detail.Run)
	}
	finalize := requireReviewGenerationOutput(t, detail, 1, "finalize_round")
	var counts struct {
		Resolved int `json:"resolved"`
		Invalid  int `json:"invalid"`
		Pending  int `json:"pending"`
	}
	if err := json.Unmarshal([]byte(finalize.OutputRef), &counts); err != nil {
		t.Fatalf("Unmarshal(finalize output) error = %v; output=%s", err, finalize.OutputRef)
	}
	if counts.Resolved != 2 || counts.Invalid != 1 || counts.Pending != 0 {
		t.Fatalf("finalize counts = %#v, want {resolved:2 invalid:1 pending:0}", counts)
	}
	fixResults := make(map[string]struct {
		triage     string
		resolution string
	})
	for _, generation := range detail.Generations {
		if generation.Generation != 1 {
			continue
		}
		for _, output := range generation.Outputs {
			if output.NodeID != "fix_batch" {
				continue
			}
			var batch struct {
				Results []struct {
					Path       string `json:"path"`
					Triage     string `json:"triage"`
					Resolution string `json:"resolution"`
				} `json:"results"`
			}
			if err := json.Unmarshal([]byte(output.OutputRef), &batch); err != nil {
				t.Fatalf("Unmarshal(fix_batch output) error = %v; output=%s", err, output.OutputRef)
			}
			if len(batch.Results) != 1 {
				t.Fatalf("fix_batch item %d results = %#v, want exactly one result", output.ItemIndex, batch.Results)
			}
			result := batch.Results[0]
			fixResults[result.Path] = struct {
				triage     string
				resolution string
			}{triage: result.Triage, resolution: result.Resolution}
		}
	}
	wantFixResults := map[string]struct {
		triage     string
		resolution string
	}{
		".compozy/tasks/review-e2e/reviews-001/issue_001.md": {triage: "valid", resolution: "fixed"},
		".compozy/tasks/review-e2e/reviews-001/issue_002.md": {triage: "invalid", resolution: "documented"},
	}
	if len(fixResults) != len(wantFixResults) {
		t.Fatalf("fix results = %#v, want %#v", fixResults, wantFixResults)
	}
	for path, want := range wantFixResults {
		if got := fixResults[path]; got != want {
			t.Fatalf("fix result %s = %#v, want %#v", path, got, want)
		}
	}
	clean := requireReviewGenerationOutput(t, detail, 2, "review")
	if clean.Status != "succeeded" || !strings.Contains(clean.OutputRef, `"issues":[]`) {
		t.Fatalf("generation-2 review = %#v, want clean success", clean)
	}
	if detail.ExecutedDefinition == nil {
		t.Fatal("review-and-fix executed definition = nil")
	}
	wantNodes := []string{
		"review", "has_issues", "write_artifacts", "fix_batches", "fix_batch", "collect_fixes", "finalize_round",
	}
	if len(detail.ExecutedDefinition.Graph.Nodes) != len(wantNodes) {
		t.Fatalf("review-and-fix executed nodes = %#v, want exactly %#v", detail.ExecutedDefinition.Graph.Nodes, wantNodes)
	}
	for index, wantNode := range wantNodes {
		if got := detail.ExecutedDefinition.Graph.Nodes[index].ID; got != wantNode {
			t.Fatalf("review-and-fix executed node[%d] = %q, want %q", index, got, wantNode)
		}
	}
}

func requireReviewGenerationOutput(
	t testing.TB,
	detail contract.LoopRunResponse,
	generation int,
	nodeID string,
) contract.LoopGenerationOutput {
	t.Helper()
	for _, current := range detail.Generations {
		if current.Generation != generation {
			continue
		}
		for _, output := range current.Outputs {
			if output.NodeID == nodeID {
				return output
			}
		}
	}
	t.Fatalf("generation %d node %q output missing: %#v", generation, nodeID, detail.Generations)
	return contract.LoopGenerationOutput{}
}

func assertReviewAndFixArtifacts(t testing.TB, workspaceRoot string) [][]byte {
	t.Helper()
	artifacts := make([][]byte, 0, 2)
	for index, fixture := range []struct {
		golden   string
		decision string
	}{
		{golden: "specific.md", decision: "VALID"},
		{golden: "general.md", decision: "INVALID"},
	} {
		path := filepath.Join(reviewRoundPath(workspaceRoot, reviewAndFixTaskName, 1), reviewIssueName(index+1))
		actual, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		golden, err := os.ReadFile(reviewGoldenPath(t, fixture.golden))
		if err != nil {
			t.Fatalf("ReadFile(%s golden) error = %v", fixture.golden, err)
		}
		createdAt := reviewFrontmatterValue(t, string(actual), "round_created_at")
		want := strings.Replace(string(golden), reviewExpectedTimestamp, createdAt, 1)
		want = strings.Replace(want, "status: pending", "status: resolved", 1)
		want = strings.Replace(want, "- Decision: `UNREVIEWED`", "- Decision: `"+fixture.decision+"`", 1)
		if string(actual) != want {
			t.Fatalf("artifact %s bytes mismatch\ngot:\n%s\nwant:\n%s", path, actual, want)
		}
		artifacts = append(artifacts, append([]byte(nil), actual...))
	}
	metaPath := filepath.Join(reviewRoundPath(workspaceRoot, reviewAndFixTaskName, 1), "_meta.md")
	if _, err := os.Stat(metaPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(%s) error = %v, want no meta file", metaPath, err)
	}
	return artifacts
}

func assertReviewArtifactsUnchanged(t testing.TB, workspaceRoot string, before [][]byte) {
	t.Helper()
	if len(before) != 2 {
		t.Fatalf("primary artifact snapshot count = %d, want 2", len(before))
	}
	for index := range before {
		path := filepath.Join(reviewRoundPath(workspaceRoot, reviewAndFixTaskName, 1), reviewIssueName(index+1))
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s after secondary run) error = %v", path, err)
		}
		if string(after) != string(before[index]) {
			t.Fatalf("secondary workspace run mutated primary artifact %s", path)
		}
	}
}

func assertReviewWriterContainment(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	workspaceRoot string,
) {
	t.Helper()
	outside := t.TempDir()
	linkedTask := filepath.Join(workspaceRoot, ".compozy", "tasks", "linked")
	if err := os.Symlink(outside, linkedTask); err != nil {
		t.Fatalf("Symlink(%s) error = %v", linkedTask, err)
	}
	for _, taskName := range []string{"../escape", "linked"} {
		input, err := json.Marshal(map[string]any{
			"task_name": taskName,
			"issues": []map[string]any{{
				"title": "Escape", "body": "Must remain contained.", "severity": "high",
			}},
		})
		if err != nil {
			t.Fatalf("Marshal(writer input) error = %v", err)
		}
		var response map[string]any
		status := loopRuntimeRawJSON(
			t,
			ctx,
			harness.HTTPClient,
			harness.HTTPURL("/api/tools/"+url.PathEscape(string(reviewArtifactWriterID))+"/invoke"),
			http.MethodPost,
			contract.ToolInvokeRequest{WorkspaceID: workspaceID, Input: input},
			&response,
		)
		if status < http.StatusBadRequest {
			t.Fatalf("writer containment task %q status = %d payload=%#v, want rejection", taskName, status, response)
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("Marshal(writer containment response) error = %v", err)
		}
		if strings.Contains(string(encoded), string(toolspkg.ErrorCodeNotFound)) ||
			strings.Contains(string(encoded), "tool_not_found") {
			t.Fatalf("writer containment task %q payload=%s, want boundary rejection instead of missing tool", taskName, encoded)
		}
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatalf("ReadDir(outside) error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("outside entries = %v, want no escaped writes", entries)
	}
}

func reviewRoundPath(workspaceRoot string, taskName string, round int) string {
	return filepath.Join(workspaceRoot, ".compozy", "tasks", taskName, fmt.Sprintf("reviews-%03d", round))
}

func reviewIssueName(number int) string {
	return fmt.Sprintf("issue_%03d.md", number)
}

func reviewGoldenPath(t testing.TB, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "extensions", "dev-cycle", "testdata", "review_artifacts", name)
}

func reviewFrontmatterValue(t testing.TB, content string, key string) string {
	t.Helper()
	prefix := key + ": "
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	t.Fatalf("frontmatter key %q missing from artifact", key)
	return ""
}
