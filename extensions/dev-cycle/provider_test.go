package devcycle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	watchpkg "github.com/compozy/agh/internal/loop/watch"
	"github.com/compozy/agh/internal/subprocess"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestRPCServerShouldCallImportTasksTool(t *testing.T) {
	t.Run("Should return structured import tasks payload over tools call", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksManifest(t, tasksDir, compozyTaskManifestVersion, []string{
			"    - from: task_01",
			"      to: task_03",
		})
		writeImportTaskFile(t, tasksDir, "task_01.md", "pending", "First task", "# First\n")
		writeImportTaskFile(t, tasksDir, "task_02.md", "done", "Second task", "# Second\n")
		writeImportTaskFile(t, tasksDir, "task_03.md", "pending", "Third task", "# Third\n")

		response := runImportTasksRPC(t, json.RawMessage(fmt.Sprintf(
			`{"pattern":%q}`,
			filepath.Join(tasksDir, "task_*.md"),
		)))
		if response.Error != nil {
			t.Fatalf("tools/call error = %#v, want result", response.Error)
		}
		var output importTasksOutput
		if err := json.Unmarshal(response.Result.Result.Structured, &output); err != nil {
			t.Fatalf("Unmarshal(structured) error = %v", err)
		}
		if output.Count != 2 || len(output.Tasks) != 2 {
			t.Fatalf("structured output = %#v, want two pending tasks", output)
		}
		if got, want := output.Tasks[0].ID, "task_01"; got != want {
			t.Fatalf("tasks[0].id = %q, want %q", got, want)
		}
		if got, want := output.Tasks[1].ID, "task_03"; got != want {
			t.Fatalf("tasks[1].id = %q, want %q", got, want)
		}
		if response.Result.Result.Bytes <= 0 || response.Result.Result.Preview == "" {
			t.Fatalf("tool result metadata = %#v, want byte count and preview", response.Result.Result)
		}
	})

	t.Run("Should surface missing pattern validation over tools call", func(t *testing.T) {
		t.Parallel()

		response := runImportTasksRPC(t, json.RawMessage(`{}`))
		if response.Error == nil {
			t.Fatalf("tools/call error = nil, want missing pattern error")
		}
		if response.Error.Code != -32010 || response.Error.Message != "The task import pattern is required." {
			t.Fatalf("tools/call error = %#v, want safe missing-pattern validation", response.Error)
		}
		var toolErr toolspkg.ToolError
		if err := json.Unmarshal(response.Error.Data, &toolErr); err != nil {
			t.Fatalf("Unmarshal(tools/call error data) error = %v", err)
		}
		if toolErr.Code != toolspkg.ErrorCodeInvalidInput || toolErr.Operator == nil ||
			toolErr.Operator.Cause == "" || toolErr.Operator.Recovery == "" {
			t.Fatalf("tools/call error data = %#v, want operator-safe invalid-input detail", toolErr)
		}
	})

	t.Run("Should surface safe recovery when the task set is missing", func(t *testing.T) {
		t.Parallel()

		workspaceDir := t.TempDir()
		pattern := filepath.Join(workspaceDir, ".compozy", "tasks", "helix-v1-launch", "task_*.md")
		response := runImportTasksRPC(t, json.RawMessage(fmt.Sprintf(`{"pattern":%q}`, pattern)))
		if response.Error == nil {
			t.Fatalf("tools/call error = nil, want missing task set error")
		}
		var toolErr toolspkg.ToolError
		if err := json.Unmarshal(response.Error.Data, &toolErr); err != nil {
			t.Fatalf("Unmarshal(tools/call error data) error = %v", err)
		}
		if got, want := toolErr.Code, toolspkg.ErrorCodeInvalidInput; got != want {
			t.Fatalf("tools/call error code = %q, want %q", got, want)
		}
		if toolErr.Operator == nil {
			t.Fatal("tools/call operator detail = nil, want safe missing-task-set detail")
		}
		if got, want := toolErr.Operator.Cause,
			"No task set matched .compozy/tasks/helix-v1-launch/task_*.md."; got != want {
			t.Fatalf("tools/call operator cause = %q, want %q", got, want)
		}
		if got, want := toolErr.Operator.Recovery,
			"Create the matching task set or correct the Loop input, then retry the run."; got != want {
			t.Fatalf("tools/call operator recovery = %q, want %q", got, want)
		}
		if strings.Contains(response.Error.Message, workspaceDir) ||
			strings.Contains(toolErr.Operator.Cause, workspaceDir) ||
			strings.Contains(toolErr.Operator.Recovery, workspaceDir) {
			t.Fatalf("tools/call error leaked workspace path %q: %#v", workspaceDir, response.Error)
		}
	})

	t.Run("Should reject malformed import task input over tools call", func(t *testing.T) {
		t.Parallel()

		response := runImportTasksRPC(t, json.RawMessage(`"not-an-object"`))
		if response.Error == nil {
			t.Fatalf("tools/call error = nil, want decode error")
		}
		if response.Error.Code != -32010 || !strings.Contains(response.Error.Message, "decode tool input") {
			t.Fatalf("tools/call error = %#v, want decode tool input validation", response.Error)
		}
	})
}

func TestRPCServerShouldServeLifecycleMethods(t *testing.T) {
	t.Run("Should initialize with advertised tool methods", func(t *testing.T) {
		t.Parallel()

		response := runProviderRPC(t, rpcMethodInitialize, subprocess.InitializeRequest{
			ProtocolVersion: "2026-01-01",
			Extension: subprocess.InitializeExtension{
				Name:    Name,
				Version: "0.1.0",
			},
			Capabilities: subprocess.InitializeCapabilities{
				Provides: []string{"tool.provider"},
			},
			Methods: subprocess.InitializeMethods{
				ExtensionServices: []string{rpcMethodToolsCall, rpcMethodProvideTools, rpcMethodWatchPoll},
			},
		})
		if response.Error != nil {
			t.Fatalf("initialize error = %#v, want result", response.Error)
		}
		var result subprocess.InitializeResponse
		if err := json.Unmarshal(response.Result, &result); err != nil {
			t.Fatalf("Unmarshal(initialize result) error = %v", err)
		}
		for _, method := range []string{
			rpcMethodHealthCheck,
			rpcMethodProvideTools,
			rpcMethodShutdown,
			rpcMethodToolsCall,
			rpcMethodWatchPoll,
		} {
			if !slices.Contains(result.ImplementedMethods, method) {
				t.Fatalf("implemented methods = %#v, want %q", result.ImplementedMethods, method)
			}
		}
		if !slices.Contains(result.WatchSourceKinds, watchKindCodeRabbitPR) {
			t.Fatalf("watch source kinds = %#v, want %q", result.WatchSourceKinds, watchKindCodeRabbitPR)
		}
	})

	t.Run("Should reject malformed initialize params", func(t *testing.T) {
		t.Parallel()

		response := runProviderRPC(t, rpcMethodInitialize, "not-an-object")
		if response.Error == nil {
			t.Fatalf("initialize error = nil, want decode error")
		}
		if response.Error.Code != -32602 || !strings.Contains(response.Error.Message, "decode initialize params") {
			t.Fatalf("initialize error = %#v, want decode params error", response.Error)
		}
	})

	t.Run("Should provide import tasks runtime descriptor", func(t *testing.T) {
		t.Parallel()

		response := runProviderRPC(t, rpcMethodProvideTools, struct{}{})
		if response.Error != nil {
			t.Fatalf("provide_tools error = %#v, want result", response.Error)
		}
		var result toolspkg.ExtensionProvideToolsResponse
		if err := json.Unmarshal(response.Result, &result); err != nil {
			t.Fatalf("Unmarshal(provide_tools result) error = %v", err)
		}
		var found bool
		for _, descriptor := range result.Tools {
			if descriptor.Handler != toolImportTasks {
				continue
			}
			found = true
			if !descriptor.ReadOnly || descriptor.Risk != toolspkg.RiskRead {
				t.Fatalf("import_tasks descriptor = %#v, want read risk", descriptor)
			}
			if strings.TrimSpace(descriptor.InputSchemaDigest) == "" ||
				strings.TrimSpace(descriptor.OutputSchemaDigest) == "" {
				t.Fatalf("import_tasks descriptor missing schema digests: %#v", descriptor)
			}
		}
		if !found {
			t.Fatalf("provide_tools descriptors = %#v, want import_tasks", result.Tools)
		}
	})

	t.Run("Should report healthy", func(t *testing.T) {
		t.Parallel()

		response := runProviderRPC(t, rpcMethodHealthCheck, struct{}{})
		if response.Error != nil {
			t.Fatalf("health_check error = %#v, want result", response.Error)
		}
		var result subprocess.HealthCheckResponse
		if err := json.Unmarshal(response.Result, &result); err != nil {
			t.Fatalf("Unmarshal(health_check result) error = %v", err)
		}
		if !result.Healthy {
			t.Fatalf("health_check healthy = false, want true")
		}
	})

	t.Run("Should acknowledge shutdown", func(t *testing.T) {
		t.Parallel()

		response := runProviderRPC(t, rpcMethodShutdown, subprocess.ShutdownRequest{Reason: "test"})
		if response.Error != nil {
			t.Fatalf("shutdown error = %#v, want result", response.Error)
		}
		var result subprocess.ShutdownResponse
		if err := json.Unmarshal(response.Result, &result); err != nil {
			t.Fatalf("Unmarshal(shutdown result) error = %v", err)
		}
		if !result.Acknowledged {
			t.Fatalf("shutdown acknowledged = false, want true")
		}
	})

	t.Run("Should reject unsupported tool handlers", func(t *testing.T) {
		t.Parallel()

		toolID, err := runtimeToolID("missing_handler")
		if err != nil {
			t.Fatalf("runtimeToolID(missing_handler) error = %v", err)
		}
		response := runProviderRPC(t, rpcMethodToolsCall, toolspkg.ExtensionToolCallRequest{
			ToolID:  toolID,
			Handler: "missing_handler",
		})
		if response.Error == nil {
			t.Fatalf("tools/call error = nil, want unsupported handler error")
		}
		if response.Error.Code != -32010 || !strings.Contains(response.Error.Message, "unsupported tool handler") {
			t.Fatalf("tools/call error = %#v, want unsupported handler error", response.Error)
		}
	})

	t.Run("Should reject malformed watch specs", func(t *testing.T) {
		t.Parallel()

		response := runProviderRPC(
			t,
			rpcMethodWatchPoll,
			watchpkg.PollRequest{Spec: json.RawMessage(`"not-an-object"`)},
		)
		if response.Error == nil {
			t.Fatalf("watch/poll error = nil, want decode error")
		}
		if response.Error.Code != -32020 || !strings.Contains(response.Error.Message, "decode watch spec") {
			t.Fatalf("watch/poll error = %#v, want decode watch spec error", response.Error)
		}
	})

	t.Run("Should reject unsupported watch kinds", func(t *testing.T) {
		t.Parallel()

		response := runProviderRPC(
			t,
			rpcMethodWatchPoll,
			watchpkg.PollRequest{Spec: json.RawMessage(`{"kind":"other"}`)},
		)
		if response.Error == nil {
			t.Fatalf("watch/poll error = nil, want unsupported kind error")
		}
		if response.Error.Code != -32020 ||
			!strings.Contains(response.Error.Message, `unsupported watch kind "other"`) {
			t.Fatalf("watch/poll error = %#v, want unsupported kind error", response.Error)
		}
	})

	t.Run("Should reject unknown methods", func(t *testing.T) {
		t.Parallel()

		response := runProviderRPC(t, "unknown/method", struct{}{})
		if response.Error == nil {
			t.Fatalf("unknown method error = nil, want method not found")
		}
		if response.Error.Code != -32601 || response.Error.Message != "method not found" {
			t.Fatalf("unknown method error = %#v, want method not found", response.Error)
		}
	})
}

func TestRPCServerShouldValidateProviderIOAndToolResultMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should require provider stdout", func(t *testing.T) {
		t.Parallel()

		err := RunProvider(context.Background(), strings.NewReader(""), nil)
		if err == nil || !strings.Contains(err.Error(), "stdout is required") {
			t.Fatalf("RunProvider(nil stdout) error = %v, want stdout validation", err)
		}
	})

	t.Run("Should require provider stdin", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		err := RunProvider(context.Background(), nil, &stdout)
		if err == nil || !strings.Contains(err.Error(), "stdin is required") {
			t.Fatalf("RunProvider(nil stdin) error = %v, want stdin validation", err)
		}
	})

	t.Run("Should truncate long tool result previews", func(t *testing.T) {
		t.Parallel()

		result, err := toolResult(map[string]string{"body": strings.Repeat("x", 400)}, time.Now())
		if err != nil {
			t.Fatalf("toolResult() error = %v", err)
		}
		if result.Bytes <= int64(len(result.Preview)) || !strings.HasSuffix(result.Preview, "...") {
			t.Fatalf("toolResult() = %#v, want truncated preview metadata", result)
		}
	})
}

func TestDefaultCommandRunnerShouldRunCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should require a command name", func(t *testing.T) {
		t.Parallel()

		runner := defaultCommandRunner{}
		_, err := runner.Run(context.Background(), " ", nil, "")
		if err == nil || !strings.Contains(err.Error(), "command name is required") {
			t.Fatalf("Run(blank) error = %v, want command name validation", err)
		}
	})

	t.Run("Should execute commands in the requested directory", func(t *testing.T) {
		t.Parallel()

		runner := defaultCommandRunner{}
		shell, err := runner.LookPath("sh")
		if err != nil {
			t.Fatalf("LookPath(sh) error = %v", err)
		}
		dir := t.TempDir()
		output, err := runner.Run(context.Background(), shell, []string{"-c", "printf '%s' \"$PWD\""}, dir)
		if err != nil {
			t.Fatalf("Run(pwd) error = %v", err)
		}
		if got := strings.TrimSpace(string(output)); got != dir {
			t.Fatalf("Run(pwd) output = %q, want %q", got, dir)
		}
	})

	t.Run("Should include stderr in failed command diagnostics", func(t *testing.T) {
		t.Parallel()

		runner := defaultCommandRunner{}
		shell, err := runner.LookPath("sh")
		if err != nil {
			t.Fatalf("LookPath(sh) error = %v", err)
		}
		_, err = runner.Run(context.Background(), shell, []string{"-c", "printf 'boom' >&2; exit 7"}, "")
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("Run(failing command) error = %v, want stderr diagnostic", err)
		}
	})
}

func TestCodeRabbitFetchInputShouldDecodeBoolLikeNitpickOption(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "omitted", raw: `{"pr":17}`, want: false},
		{name: "boolean", raw: `{"pr":17,"include_nitpicks":true}`, want: true},
		{name: "string", raw: `{"pr":17,"include_nitpicks":" true "}`, want: true},
		{name: "blank string", raw: `{"pr":17,"include_nitpicks":" "}`, want: false},
	}
	for _, tc := range cases {
		t.Run("Should decode "+tc.name, func(t *testing.T) {
			t.Parallel()

			var input codeRabbitFetchInput
			if err := json.Unmarshal([]byte(tc.raw), &input); err != nil {
				t.Fatalf("Unmarshal(fetch input) error = %v", err)
			}
			if input.IncludeNitpicks != tc.want {
				t.Fatalf("IncludeNitpicks = %t, want %t", input.IncludeNitpicks, tc.want)
			}
		})
	}

	t.Run("Should reject unsupported boolean values", func(t *testing.T) {
		t.Parallel()

		var input codeRabbitFetchInput
		err := json.Unmarshal([]byte(`{"pr":17,"include_nitpicks":1}`), &input)
		if err == nil || !strings.Contains(err.Error(), "unsupported boolean value") {
			t.Fatalf("Unmarshal(fetch input) error = %v, want unsupported boolean validation", err)
		}
	})

	t.Run("Should reject invalid boolean strings", func(t *testing.T) {
		t.Parallel()

		var input codeRabbitFetchInput
		err := json.Unmarshal([]byte(`{"pr":17,"include_nitpicks":"maybe"}`), &input)
		if err == nil || !strings.Contains(err.Error(), "include_nitpicks") {
			t.Fatalf("Unmarshal(fetch input) error = %v, want include_nitpicks validation", err)
		}
	})
}

func TestCodeRabbitInputHelpersShouldNormalizePRAndRemotes(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize supported pull request values", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			want  string
		}{
			{name: "Should normalize float values", value: float64(17), want: "17"},
			{name: "Should normalize integer values", value: 18, want: "18"},
			{name: "Should normalize prefixed string values", value: "#19", want: "19"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, err := normalizePR(tc.value)
				if err != nil {
					t.Fatalf("normalizePR(%#v) error = %v", tc.value, err)
				}
				if got != tc.want {
					t.Fatalf("normalizePR(%#v) = %q, want %q", tc.value, got, tc.want)
				}
			})
		}
	})

	t.Run("Should reject unsupported pull request values", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			value   any
			wantErr string
		}{
			{name: "Should reject fractional values", value: float64(1.5), wantErr: "positive integer"},
			{name: "Should reject zero", value: 0, wantErr: "positive integer"},
			{name: "Should reject nonnumeric strings", value: "abc", wantErr: "must be numeric"},
			{name: "Should reject unsupported types", value: []string{"17"}, wantErr: "unsupported pr value"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, err := normalizePR(tc.value)
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("normalizePR(%#v) = %q, %v; want %q validation", tc.value, got, err, tc.wantErr)
				}
			})
		}
	})

	t.Run("Should parse GitHub remote URLs", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			remote string
			want   string
		}{
			{name: "Should parse SSH remotes", remote: "git@github.com:acme/repo.git", want: "acme/repo"},
			{name: "Should parse HTTPS remotes", remote: "https://github.com/acme/repo.git", want: "acme/repo"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, err := parseGitHubRemote(tc.remote)
				if err != nil {
					t.Fatalf("parseGitHubRemote(%q) error = %v", tc.remote, err)
				}
				if got != tc.want {
					t.Fatalf("parseGitHubRemote(%q) = %q, want %q", tc.remote, got, tc.want)
				}
			})
		}
	})

	t.Run("Should reject non-GitHub remotes", func(t *testing.T) {
		t.Parallel()

		got, err := parseGitHubRemote("https://example.com/acme/repo.git")
		if err == nil || !strings.Contains(err.Error(), "origin remote is not github.com") {
			t.Fatalf("parseGitHubRemote(non-github) = %q, %v; want host validation", got, err)
		}
	})
}

func TestCodeRabbitOrderingHelpersShouldPreferNewestProviderEvents(t *testing.T) {
	t.Parallel()

	t.Run("Should choose the newest CodeRabbit commit status", func(t *testing.T) {
		t.Parallel()

		status, ok := latestCodeRabbitCommitStatus([]commitStatus{
			{Context: "other", UpdatedAt: "2026-07-08T13:00:00Z", State: "success"},
			{Context: codeRabbitCommitStatusContext, UpdatedAt: "2026-07-08T12:00:00Z", State: "pending"},
			{Context: codeRabbitCommitStatusContext, CreatedAt: "2026-07-08T14:00:00Z", State: "success"},
		})
		if !ok || status.State != "success" {
			t.Fatalf("latestCodeRabbitCommitStatus() = %#v, %t; want newest success", status, ok)
		}
	})

	t.Run("Should compare commit statuses by timestamp", func(t *testing.T) {
		t.Parallel()

		current := commitStatus{UpdatedAt: "2026-07-08T12:00:00Z"}
		if !commitStatusIsNewer(commitStatus{UpdatedAt: "2026-07-08T12:01:00Z"}, current) {
			t.Fatalf("commitStatusIsNewer(newer, current) = false, want true")
		}
		if commitStatusIsNewer(commitStatus{UpdatedAt: "not-a-time"}, current) {
			t.Fatalf("commitStatusIsNewer(invalid, current) = true, want false")
		}
	})

	t.Run("Should choose the newest CodeRabbit review with id tiebreaks", func(t *testing.T) {
		t.Parallel()

		newer := pullRequestReview{
			ID:          12,
			SubmittedAt: "2026-07-08T12:00:00Z",
		}
		newer.User.Login = codeRabbitBotLogin
		olderSameTime := pullRequestReview{
			ID:          11,
			SubmittedAt: "2026-07-08T12:00:00Z",
		}
		olderSameTime.User.Login = codeRabbitBotLogin
		otherBot := pullRequestReview{ID: 99, SubmittedAt: "2026-07-08T13:00:00Z"}
		otherBot.User.Login = "someone"

		review, ok := latestCodeRabbitReview([]pullRequestReview{olderSameTime, otherBot, newer})
		if !ok || review.ID != 12 {
			t.Fatalf("latestCodeRabbitReview() = %#v, %t; want review 12", review, ok)
		}
		if !reviewIsNewer(newer, olderSameTime) {
			t.Fatalf("reviewIsNewer(newer, olderSameTime) = false, want true")
		}
		if compareReviewIDs("abc", "99") <= 0 {
			t.Fatalf("compareReviewIDs(string fallback) <= 0, want lexical fallback ordering")
		}
	})

	t.Run("Should choose newer review body issues by submitted time and review id", func(t *testing.T) {
		t.Parallel()

		current := codeRabbitIssue{
			SourceReviewID:          "9",
			SourceReviewSubmittedAt: "2026-07-08T12:00:00Z",
		}
		if !reviewBodyCommentIssueIsNewer(codeRabbitIssue{
			SourceReviewID:          "8",
			SourceReviewSubmittedAt: "2026-07-08T12:01:00Z",
		}, current) {
			t.Fatalf("reviewBodyCommentIssueIsNewer(newer timestamp) = false, want true")
		}
		if !reviewBodyCommentIssueIsNewer(codeRabbitIssue{
			SourceReviewID:          "10",
			SourceReviewSubmittedAt: "2026-07-08T12:00:00Z",
		}, current) {
			t.Fatalf("reviewBodyCommentIssueIsNewer(review id tiebreak) = false, want true")
		}
	})
}

func TestCodeRabbitReviewBodyHelpersShouldNormalizeCommentMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should parse severity summaries", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			summary string
			want    string
		}{
			{summary: "Nitpick comments", want: reviewBodyCommentSeverityNitpick},
			{summary: "Minor comment", want: reviewBodyCommentSeverityMinor},
			{summary: "Major comments", want: reviewBodyCommentSeverityMajor},
			{summary: "Critical comments", want: reviewBodyCommentSeverityCritical},
		}
		for _, tc := range cases {
			got, ok := reviewBodyCommentSeverity(tc.summary)
			if !ok || got != tc.want {
				t.Fatalf("reviewBodyCommentSeverity(%q) = %q, %t; want %q", tc.summary, got, ok, tc.want)
			}
		}
		if got, ok := reviewBodyCommentSeverity("general summary"); ok || got != "" {
			t.Fatalf("reviewBodyCommentSeverity(unknown) = %q, %t; want no match", got, ok)
		}
	})

	t.Run("Should parse standalone titles and body text", func(t *testing.T) {
		t.Parallel()

		title, body := parseReviewBodyCommentSection(strings.Join([]string{
			"`12-13`:",
			"severity: critical",
			"**Use the bounded context helper**",
			"",
			"Please avoid open-ended context usage.",
		}, "\n"))
		if title != "Use the bounded context helper" {
			t.Fatalf("title = %q, want standalone title", title)
		}
		if !strings.Contains(body, "open-ended context") {
			t.Fatalf("body = %q, want normalized body", body)
		}
		if severity, ok := reviewBodyCommentSectionSeverity("`12-13`:\nseverity: critical\n**Title**"); !ok ||
			severity != reviewBodyCommentSeverityCritical {
			t.Fatalf("reviewBodyCommentSectionSeverity() = %q, %t; want critical", severity, ok)
		}
	})

	t.Run("Should strip wrapper detail blocks without losing surrounding text", func(t *testing.T) {
		t.Parallel()

		input := "before\n<details><summary>hidden</summary>remove me</details>\nafter"
		output := stripTopLevelDetailsBlocks(input)
		if strings.Contains(output, "remove me") ||
			!strings.Contains(output, "before") ||
			!strings.Contains(output, "after") {
			t.Fatalf("stripTopLevelDetailsBlocks() = %q, want only surrounding text", output)
		}
		if !reviewBodyCommentOutsideDiffRange("Outside diff range comments") {
			t.Fatalf("reviewBodyCommentOutsideDiffRange() = false, want true")
		}
	})
}

func TestCodeRabbitProviderShouldSurfaceProviderFailures(t *testing.T) {
	t.Run("Should fail when gh is unavailable", func(t *testing.T) {
		t.Parallel()

		provider := newCodeRabbitProvider(&recordingCommandRunner{
			lookPathErrs: map[string]error{"gh": errors.New("not found")},
		})

		_, err := provider.FetchUnresolved(context.Background(), codeRabbitFetchInput{PR: 17})
		if err == nil || !strings.Contains(err.Error(), "gh executable is required") {
			t.Fatalf("FetchUnresolved() error = %v, want missing gh diagnostic", err)
		}
	})

	t.Run("Should surface GitHub GraphQL auth errors", func(t *testing.T) {
		t.Parallel()

		runner := &recordingCommandRunner{
			lookPathResults: map[string]string{"gh": "/usr/bin/gh"},
			runResults: map[string][]byte{
				commandKey(
					"gh",
					"repo",
					"view",
					"--json",
					"nameWithOwner",
					"-q",
					".nameWithOwner",
				): []byte("acme/repo\n"),
				commandKey(
					"gh",
					"api",
					"graphql",
					"-f",
					"query="+fetchPRQueryForTest(),
					"-F",
					"owner=acme",
					"-F",
					"repo=repo",
					"-F",
					"pr=17",
				): []byte(`{"errors":[{"message":"HTTP 401: Bad credentials"}]}`),
			},
		}
		provider := newCodeRabbitProvider(runner)

		_, err := provider.FetchUnresolved(context.Background(), codeRabbitFetchInput{PR: 17})
		if err == nil || !strings.Contains(err.Error(), "GitHub GraphQL error: HTTP 401") {
			t.Fatalf("FetchUnresolved() error = %v, want GraphQL auth diagnostic", err)
		}
	})

	t.Run("Should resolve only accepted provider threads once", func(t *testing.T) {
		t.Parallel()

		runner := &recordingCommandRunner{
			lookPathResults: map[string]string{"gh": "/usr/bin/gh"},
		}
		provider := newCodeRabbitProvider(runner)
		input := codeRabbitResolveInput{
			PR: "17",
			Issues: []codeRabbitIssue{
				{ID: "issue-a", ProviderRef: "thread-a"},
				{ID: "issue-b", ProviderRef: "thread-b"},
				{ID: "issue-c", ProviderRef: "thread-a"},
			},
			Results: []codeRabbitFixEntry{
				{ID: "issue-a", Resolution: "fixed"},
				{ProviderRef: "thread-b", Triage: "invalid"},
			},
		}

		output, err := provider.ResolveThreads(context.Background(), input)
		if err != nil {
			t.Fatalf("ResolveThreads() error = %v", err)
		}
		if output.ResolvedCount != 1 || !slices.Equal(output.ResolvedThreads, []string{"thread-a"}) {
			t.Fatalf("ResolveThreads() output = %#v, want only thread-a resolved once", output)
		}
		if got, want := len(runner.calls), 1; got != want {
			t.Fatalf("command calls = %d, want %d", got, want)
		}
		if got := strings.Join(runner.calls[0].args, " "); !strings.Contains(got, "id=thread-a") {
			t.Fatalf("resolve command args = %q, want thread-a mutation", got)
		}
	})

	t.Run("Should reject empty resolution results for fetched issues", func(t *testing.T) {
		t.Parallel()

		runner := &recordingCommandRunner{
			lookPathResults: map[string]string{"gh": "/usr/bin/gh"},
		}
		provider := newCodeRabbitProvider(runner)

		_, err := provider.ResolveThreads(context.Background(), codeRabbitResolveInput{
			PR: "17",
			Issues: []codeRabbitIssue{
				{ID: "issue-a", ProviderRef: "thread-a"},
			},
		})
		if !errors.Is(err, watchpkg.ErrSpecInvalid) {
			t.Fatalf("ResolveThreads() error = %v, want ErrSpecInvalid", err)
		}
		if runner.called("gh", "api") {
			t.Fatalf("gh api was called for empty results: %#v", runner.calls)
		}
	})

	t.Run("Should reject triage-only valid results", func(t *testing.T) {
		t.Parallel()

		runner := &recordingCommandRunner{
			lookPathResults: map[string]string{"gh": "/usr/bin/gh"},
		}
		provider := newCodeRabbitProvider(runner)

		_, err := provider.ResolveThreads(context.Background(), codeRabbitResolveInput{
			PR: "17",
			Issues: []codeRabbitIssue{
				{ID: "issue-a", ProviderRef: "thread-a"},
			},
			Results: []codeRabbitFixEntry{
				{ID: "issue-a", Triage: "valid"},
			},
		})
		if !errors.Is(err, watchpkg.ErrSpecInvalid) {
			t.Fatalf("ResolveThreads() error = %v, want ErrSpecInvalid", err)
		}
		if runner.called("gh", "api") {
			t.Fatalf("gh api was called for triage-only result: %#v", runner.calls)
		}
	})

	t.Run("Should resolve documented provider refs without issue ids", func(t *testing.T) {
		t.Parallel()

		runner := &recordingCommandRunner{
			lookPathResults: map[string]string{"gh": "/usr/bin/gh"},
		}
		provider := newCodeRabbitProvider(runner)

		output, err := provider.ResolveThreads(context.Background(), codeRabbitResolveInput{
			PR: "17",
			Issues: []codeRabbitIssue{
				{ID: "issue-a", ProviderRef: "thread-a"},
			},
			Results: []codeRabbitFixEntry{
				{ProviderRef: "thread-a", Resolution: "documented"},
			},
		})
		if err != nil {
			t.Fatalf("ResolveThreads() error = %v", err)
		}
		if output.ResolvedCount != 1 || !slices.Equal(output.ResolvedThreads, []string{"thread-a"}) {
			t.Fatalf("ResolveThreads() output = %#v, want thread-a resolved", output)
		}
	})

	t.Run("Should skip synthetic nitpick refs when resolving GitHub review threads", func(t *testing.T) {
		t.Parallel()

		runner := &recordingCommandRunner{
			lookPathResults: map[string]string{"gh": "/usr/bin/gh"},
		}
		provider := newCodeRabbitProvider(runner)

		output, err := provider.ResolveThreads(context.Background(), codeRabbitResolveInput{
			PR: "17",
			Issues: []codeRabbitIssue{
				{ID: "issue-thread", ProviderRef: "thread-a"},
				{ID: "issue-nitpick", ProviderRef: "review:901,nitpick_hash:abc123"},
			},
			Results: []codeRabbitFixEntry{
				{ID: "issue-thread", Resolution: "fixed"},
				{ID: "issue-nitpick", Resolution: "fixed"},
			},
		})
		if err != nil {
			t.Fatalf("ResolveThreads() error = %v", err)
		}
		if output.ResolvedCount != 1 || !slices.Equal(output.ResolvedThreads, []string{"thread-a"}) {
			t.Fatalf("ResolveThreads() output = %#v, want only real review thread resolved", output)
		}
		if got, want := len(runner.calls), 1; got != want {
			t.Fatalf("command calls = %d, want %d", got, want)
		}
		args := strings.Join(runner.calls[0].args, " ")
		if !strings.Contains(args, "id=thread-a") || strings.Contains(args, "review:901") {
			t.Fatalf("resolve command args = %q, want only thread-a mutation", args)
		}
	})

	t.Run("Should accept nitpick-only remediation without GitHub thread mutation", func(t *testing.T) {
		t.Parallel()

		runner := &recordingCommandRunner{
			lookPathResults: map[string]string{"gh": "/usr/bin/gh"},
		}
		provider := newCodeRabbitProvider(runner)

		output, err := provider.ResolveThreads(context.Background(), codeRabbitResolveInput{
			PR: "17",
			Issues: []codeRabbitIssue{
				{ID: "issue-nitpick", ProviderRef: "review:901,nitpick_hash:abc123"},
			},
			Results: []codeRabbitFixEntry{
				{ID: "issue-nitpick", Resolution: "documented"},
			},
		})
		if err != nil {
			t.Fatalf("ResolveThreads() error = %v", err)
		}
		if output.ResolvedCount != 0 || len(output.ResolvedThreads) != 0 {
			t.Fatalf("ResolveThreads() output = %#v, want no GitHub thread resolutions", output)
		}
		if runner.called("gh", "api") {
			t.Fatalf("gh api was called for synthetic nitpick ref: %#v", runner.calls)
		}
	})
}

func TestCodeRabbitProviderShouldFetchReviewItems(t *testing.T) {
	t.Run("Should fetch unresolved review threads without opt-in nitpicks", func(t *testing.T) {
		t.Parallel()

		runner := codeRabbitFetchRunner(t, codeRabbitGraphQLWithThread(), codeRabbitReviewsWithNitpick(t))
		provider := newCodeRabbitProvider(runner)

		output, err := provider.FetchUnresolved(context.Background(), codeRabbitFetchInput{PR: 17})
		if err != nil {
			t.Fatalf("FetchUnresolved() error = %v", err)
		}
		if output.UnresolvedCount != 1 {
			t.Fatalf("FetchUnresolved() unresolved_count = %d, want 1", output.UnresolvedCount)
		}
		issue := output.Issues[0]
		if issue.ProviderRef != "thread-a" || issue.Author != codeRabbitBotLogin || issue.Severity != "review" {
			t.Fatalf("FetchUnresolved() issue = %#v, want CodeRabbit thread issue", issue)
		}
		if runner.calledWithArg("gh", "repos/acme/repo/pulls/17/reviews?per_page=100&page=1") {
			t.Fatalf("pull request reviews were fetched without include_nitpicks: %#v", runner.calls)
		}
	})

	t.Run("Should include review body nitpicks only when requested", func(t *testing.T) {
		t.Parallel()

		runner := codeRabbitFetchRunner(t, codeRabbitGraphQLWithThread(), codeRabbitReviewsWithNitpick(t))
		provider := newCodeRabbitProvider(runner)

		output, err := provider.FetchUnresolved(context.Background(), codeRabbitFetchInput{
			PR:              17,
			IncludeNitpicks: true,
		})
		if err != nil {
			t.Fatalf("FetchUnresolved() error = %v", err)
		}
		if output.UnresolvedCount != 2 {
			t.Fatalf("FetchUnresolved() unresolved_count = %d, want 2", output.UnresolvedCount)
		}
		nitpick := output.Issues[0]
		if output.Issues[1].Severity == reviewBodyCommentSeverityNitpick {
			nitpick = output.Issues[1]
		}
		if nitpick.Severity != reviewBodyCommentSeverityNitpick {
			t.Fatalf("FetchUnresolved() issues = %#v, want nitpick issue", output.Issues)
		}
		if nitpick.File != "internal/foo.go" || nitpick.Line != 12 {
			t.Fatalf("nitpick location = %s:%d, want internal/foo.go:12", nitpick.File, nitpick.Line)
		}
		if nitpick.SourceReviewID != "901" || nitpick.SourceReviewSubmittedAt != "2026-07-07T12:00:00Z" {
			t.Fatalf("nitpick source metadata = %#v, want review metadata", nitpick)
		}
		if !strings.HasPrefix(nitpick.ProviderRef, "review:901,nitpick_hash:") {
			t.Fatalf("nitpick provider_ref = %q, want review/hash provider ref", nitpick.ProviderRef)
		}
	})
}

func TestCodeRabbitProviderShouldPollCurrentReviewStatus(t *testing.T) {
	t.Run("Should mark current reviewed after successful CodeRabbit status on local HEAD", func(t *testing.T) {
		t.Parallel()

		runner := codeRabbitPollRunner(
			"17",
			"head-sha",
			"head-sha",
			codeRabbitStatuses("success", "finished"),
			codeRabbitReviews("901", "head-sha", "COMMENTED"),
		)
		provider := newCodeRabbitProvider(runner)

		response, err := provider.Poll(context.Background(), watchpkg.PollRequest{}, codeRabbitWatchSpec{
			PR:          17,
			QuietPeriod: "20s",
		})
		if err != nil {
			t.Fatalf("Poll() error = %v", err)
		}
		if !response.Ready {
			t.Fatalf("Poll() ready = false, want true")
		}
		payload := decodeCodeRabbitWatchPayload(t, response.Payload)
		if payload.ProviderState.State != codeRabbitWatchCurrentReviewed {
			t.Fatalf("provider state = %#v, want current reviewed", payload.ProviderState)
		}
		if payload.Review.HeadSHA != "head-sha" ||
			payload.Review.LocalHeadSHA != "head-sha" ||
			payload.Review.ReviewCommitSHA != "head-sha" {
			t.Fatalf("review payload = %#v, want matching head/local/review SHAs", payload.Review)
		}
		if response.SettledAt == nil {
			t.Fatalf("Poll() settled_at = nil, want quiet-period deadline")
		}
	})

	t.Run("Should confirm ready state even when digest did not change", func(t *testing.T) {
		t.Parallel()

		runner := codeRabbitPollRunner(
			"17",
			"head-sha",
			"head-sha",
			codeRabbitStatuses("success", "finished"),
			codeRabbitReviews("901", "head-sha", "COMMENTED"),
		)
		provider := newCodeRabbitProvider(runner)

		first, err := provider.Poll(context.Background(), watchpkg.PollRequest{}, codeRabbitWatchSpec{
			PR:          17,
			QuietPeriod: "20s",
		})
		if err != nil {
			t.Fatalf("Poll(first) error = %v", err)
		}
		if !first.Ready || first.StateDigest == "" {
			t.Fatalf("Poll(first) = %#v, want ready response with digest", first)
		}
		second, err := provider.Poll(
			context.Background(),
			watchpkg.PollRequest{ExpectedStateDigest: first.StateDigest},
			codeRabbitWatchSpec{PR: 17, QuietPeriod: "20s"},
		)
		if err != nil {
			t.Fatalf("Poll(second) error = %v", err)
		}
		if !second.Ready {
			t.Fatalf("Poll(second) ready = false, want ready confirmation for unchanged digest")
		}
		if len(second.Payload) == 0 {
			t.Fatalf("Poll(second) payload empty, want confirmation payload")
		}
		if second.SettledAt == nil {
			t.Fatalf("Poll(second) settled_at = nil, want quiet-period deadline")
		}
	})

	t.Run("Should keep stale review commits not ready", func(t *testing.T) {
		t.Parallel()

		runner := codeRabbitPollRunner(
			"17",
			"head-sha",
			"head-sha",
			codeRabbitStatuses("success", "finished"),
			codeRabbitReviews("901", "old-sha", "COMMENTED"),
		)
		provider := newCodeRabbitProvider(runner)

		response, err := provider.Poll(context.Background(), watchpkg.PollRequest{}, codeRabbitWatchSpec{PR: 17})
		if err != nil {
			t.Fatalf("Poll() error = %v", err)
		}
		if response.Ready {
			t.Fatalf("Poll() ready = true, want stale review to block readiness")
		}
		payload := decodeCodeRabbitWatchPayload(t, response.Payload)
		if payload.ProviderState.State != codeRabbitWatchStale {
			t.Fatalf("provider state = %#v, want stale", payload.ProviderState)
		}
	})

	t.Run("Should keep pending CodeRabbit status not ready", func(t *testing.T) {
		t.Parallel()

		runner := codeRabbitPollRunner(
			"17",
			"head-sha",
			"head-sha",
			codeRabbitStatuses("pending", "reviewing"),
			codeRabbitReviews("901", "head-sha", "COMMENTED"),
		)
		provider := newCodeRabbitProvider(runner)

		response, err := provider.Poll(context.Background(), watchpkg.PollRequest{}, codeRabbitWatchSpec{PR: 17})
		if err != nil {
			t.Fatalf("Poll() error = %v", err)
		}
		if response.Ready {
			t.Fatalf("Poll() ready = true, want pending status to block readiness")
		}
		payload := decodeCodeRabbitWatchPayload(t, response.Payload)
		if payload.ProviderState.State != codeRabbitWatchPending {
			t.Fatalf("provider state = %#v, want pending", payload.ProviderState)
		}
	})

	t.Run("Should surface failed CodeRabbit status as provider error", func(t *testing.T) {
		t.Parallel()

		runner := codeRabbitPollRunner(
			"17",
			"head-sha",
			"head-sha",
			codeRabbitStatuses("failure", "review failed"),
			codeRabbitReviews("901", "head-sha", "COMMENTED"),
		)
		provider := newCodeRabbitProvider(runner)

		_, err := provider.Poll(context.Background(), watchpkg.PollRequest{}, codeRabbitWatchSpec{PR: 17})
		if err == nil || !strings.Contains(err.Error(), "coderabbit status \"failure\"") {
			t.Fatalf("Poll() error = %v, want failed status diagnostic", err)
		}
	})

	t.Run("Should block readiness when local HEAD differs from PR head", func(t *testing.T) {
		t.Parallel()

		runner := codeRabbitPollRunner(
			"17",
			"head-sha",
			"local-sha",
			codeRabbitStatuses("success", "finished"),
			codeRabbitReviews("901", "head-sha", "COMMENTED"),
		)
		provider := newCodeRabbitProvider(runner)

		response, err := provider.Poll(context.Background(), watchpkg.PollRequest{}, codeRabbitWatchSpec{PR: 17})
		if err != nil {
			t.Fatalf("Poll() error = %v", err)
		}
		if response.Ready {
			t.Fatalf("Poll() ready = true, want local HEAD mismatch to block readiness")
		}
		payload := decodeCodeRabbitWatchPayload(t, response.Payload)
		if payload.ProviderState.State != codeRabbitWatchLocalHeadMismatch {
			t.Fatalf("provider state = %#v, want local head mismatch", payload.ProviderState)
		}
	})
}

func TestGitProviderShouldGuardPushes(t *testing.T) {
	t.Run("Should require a changed HEAD before guarded push", func(t *testing.T) {
		t.Parallel()

		runner := gitRunnerForPushTest("feature", "abc123")
		provider := newGitProvider(runner)

		_, err := provider.Push(context.Background(), gitPushInput{
			Remote:              "origin",
			RequireHeadAdvanced: true,
			ExpectedHead:        "abc123",
		})
		if err == nil || !strings.Contains(err.Error(), "HEAD did not advance") {
			t.Fatalf("Push() error = %v, want head-advance guard", err)
		}
		if runner.called("git", "push") {
			t.Fatalf("git push was called despite unchanged HEAD: %#v", runner.calls)
		}
	})

	t.Run("Should require expected_head when the head guard is enabled", func(t *testing.T) {
		t.Parallel()

		runner := gitRunnerForPushTest("feature", "def456")
		provider := newGitProvider(runner)

		_, err := provider.Push(context.Background(), gitPushInput{
			Remote:              "origin",
			RequireHeadAdvanced: true,
		})
		if err == nil || !strings.Contains(err.Error(), "expected_head is required") {
			t.Fatalf("Push() error = %v, want expected_head validation", err)
		}
		if runner.called("git", "push") {
			t.Fatalf("git push was called without expected_head: %#v", runner.calls)
		}
	})

	t.Run("Should separate push options from remote and branch", func(t *testing.T) {
		t.Parallel()

		runner := gitRunnerForPushTest("feature", "def456")
		provider := newGitProvider(runner)

		output, err := provider.Push(context.Background(), gitPushInput{Remote: "origin"})
		if err != nil {
			t.Fatalf("Push() error = %v", err)
		}
		if !output.Pushed || output.Branch != "feature" || output.Head != "def456" {
			t.Fatalf("Push() output = %#v, want pushed feature at def456", output)
		}
		if len(runner.calls) != 3 {
			t.Fatalf("command calls = %#v, want branch/head/push", runner.calls)
		}
		if got, want := strings.Join(runner.calls[2].args, " "), "push -- origin feature"; got != want {
			t.Fatalf("git push args = %q, want %q", got, want)
		}
	})
}

type rpcRawResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcToolCallResponse struct {
	JSONRPC string                             `json:"jsonrpc"`
	ID      json.RawMessage                    `json:"id"`
	Result  toolspkg.ExtensionToolCallResponse `json:"result"`
	Error   *rpcError                          `json:"error,omitempty"`
}

func runImportTasksRPC(t *testing.T, input json.RawMessage) rpcToolCallResponse {
	t.Helper()
	toolID, err := runtimeToolID(toolImportTasks)
	if err != nil {
		t.Fatalf("runtimeToolID(%q) error = %v", toolImportTasks, err)
	}
	params := toolspkg.ExtensionToolCallRequest{
		ToolID:  toolID,
		Handler: toolImportTasks,
		Input:   input,
	}
	request := struct {
		JSONRPC string                            `json:"jsonrpc"`
		ID      int                               `json:"id"`
		Method  string                            `json:"method"`
		Params  toolspkg.ExtensionToolCallRequest `json:"params"`
	}{
		JSONRPC: jsonRPCVersion,
		ID:      1,
		Method:  rpcMethodToolsCall,
		Params:  params,
	}
	line, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Marshal(rpc request) error = %v", err)
	}
	rawResponse := runProviderRPCLine(t, string(line))
	var response rpcToolCallResponse
	if err := json.Unmarshal(rawResponse, &response); err != nil {
		t.Fatalf("Unmarshal(rpc response %q) error = %v", string(rawResponse), err)
	}
	if response.JSONRPC != jsonRPCVersion {
		t.Fatalf("rpc jsonrpc = %q, want %q", response.JSONRPC, jsonRPCVersion)
	}
	return response
}

func runProviderRPC(t *testing.T, method string, params any) rpcRawResponse {
	t.Helper()
	request := struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{
		JSONRPC: jsonRPCVersion,
		ID:      1,
		Method:  method,
		Params:  params,
	}
	line, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Marshal(rpc request) error = %v", err)
	}
	rawResponse := runProviderRPCLine(t, string(line))
	var response rpcRawResponse
	if err := json.Unmarshal(rawResponse, &response); err != nil {
		t.Fatalf("Unmarshal(rpc response %q) error = %v", string(rawResponse), err)
	}
	if response.JSONRPC != jsonRPCVersion {
		t.Fatalf("rpc jsonrpc = %q, want %q", response.JSONRPC, jsonRPCVersion)
	}
	return response
}

func runProviderRPCLine(t *testing.T, line string) []byte {
	t.Helper()
	var stdout bytes.Buffer
	if err := RunProvider(context.Background(), strings.NewReader(line+"\n"), &stdout); err != nil {
		t.Fatalf("RunProvider() error = %v", err)
	}
	return bytes.TrimSpace(stdout.Bytes())
}

type recordedCommand struct {
	name string
	args []string
}

type recordingCommandRunner struct {
	lookPathResults map[string]string
	lookPathErrs    map[string]error
	runResults      map[string][]byte
	runErrs         map[string]error
	calls           []recordedCommand
}

func (r *recordingCommandRunner) LookPath(file string) (string, error) {
	if err := r.lookPathErrs[file]; err != nil {
		return "", err
	}
	if path := strings.TrimSpace(r.lookPathResults[file]); path != "" {
		return path, nil
	}
	return "/usr/bin/" + file, nil
}

func (r *recordingCommandRunner) Run(
	_ context.Context,
	name string,
	args []string,
	_ string,
) ([]byte, error) {
	r.calls = append(r.calls, recordedCommand{name: name, args: append([]string(nil), args...)})
	key := commandKey(name, args...)
	if err := r.runErrs[key]; err != nil {
		return nil, err
	}
	if output, ok := r.runResults[key]; ok {
		return append([]byte(nil), output...), nil
	}
	return []byte{}, nil
}

func (r *recordingCommandRunner) called(name string, firstArg string) bool {
	for _, call := range r.calls {
		if call.name == name && len(call.args) > 0 && call.args[0] == firstArg {
			return true
		}
	}
	return false
}

func (r *recordingCommandRunner) calledWithArg(name string, arg string) bool {
	for _, call := range r.calls {
		if call.name != name {
			continue
		}
		if slices.Contains(call.args, arg) {
			return true
		}
	}
	return false
}

func gitRunnerForPushTest(branch string, head string) *recordingCommandRunner {
	return &recordingCommandRunner{
		lookPathResults: map[string]string{"git": "/usr/bin/git"},
		runResults: map[string][]byte{
			commandKey("git", "rev-parse", "--abbrev-ref", gitHeadRef): []byte(branch + "\n"),
			commandKey("git", "rev-parse", gitHeadRef):                 []byte(head + "\n"),
		},
	}
}

func commandKey(name string, args ...string) string {
	return name + "\x00" + strings.Join(args, "\x00")
}

func fetchPRQueryForTest() string {
	return `
query($owner:String!,$repo:String!,$pr:Int!){
  repository(owner:$owner,name:$repo){
    pullRequest(number:$pr){
      headRefOid
      reviewThreads(first:100){nodes{id isResolved comments(first:20){nodes{body path line author{login} createdAt}}}}
    }
  }
}`
}

func codeRabbitRepoViewKey() string {
	return commandKey("gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner")
}

func codeRabbitGraphQLKey(pr string) string {
	return commandKey(
		"gh",
		"api",
		"graphql",
		"-f",
		"query="+fetchPRQueryForTest(),
		"-F",
		"owner=acme",
		"-F",
		"repo=repo",
		"-F",
		"pr="+pr,
	)
}

func codeRabbitFetchRunner(t *testing.T, graphQL string, reviews string) *recordingCommandRunner {
	t.Helper()
	return &recordingCommandRunner{
		lookPathResults: map[string]string{"gh": "/usr/bin/gh"},
		runResults: map[string][]byte{
			codeRabbitRepoViewKey():    []byte("acme/repo\n"),
			codeRabbitGraphQLKey("17"): []byte(graphQL),
			commandKey(
				"gh",
				"api",
				"repos/acme/repo/pulls/17/reviews?per_page=100&page=1",
			): []byte(reviews),
		},
	}
}

func codeRabbitPollRunner(
	pr string,
	head string,
	localHead string,
	statuses string,
	reviews string,
) *recordingCommandRunner {
	return &recordingCommandRunner{
		lookPathResults: map[string]string{"gh": "/usr/bin/gh", "git": "/usr/bin/git"},
		runResults: map[string][]byte{
			codeRabbitRepoViewKey(): []byte("acme/repo\n"),
			codeRabbitGraphQLKey(pr): fmt.Appendf(nil,
				`{"data":{"repository":{"pullRequest":{"headRefOid":%q,"reviewThreads":{"nodes":[]}}}}}`,
				head,
			),
			commandKey("git", "rev-parse", "HEAD"): []byte(localHead + "\n"),
			commandKey(
				"gh",
				"api",
				fmt.Sprintf("repos/acme/repo/commits/%s/statuses?per_page=100&page=1", head),
			): []byte(statuses),
			commandKey(
				"gh",
				"api",
				fmt.Sprintf("repos/acme/repo/pulls/%s/reviews?per_page=100&page=1", pr),
			): []byte(reviews),
		},
	}
}

func codeRabbitGraphQLWithThread() string {
	return `{"data":{"repository":{"pullRequest":{"headRefOid":"head-sha","reviewThreads":{"nodes":[{"id":"thread-a","isResolved":false,"comments":{"nodes":[{"body":"Fix the production path","path":"internal/foo.go","line":9,"author":{"login":"coderabbitai[bot]"},"createdAt":"2026-07-07T12:00:00Z"}]}}]}}}}}`
}

func codeRabbitReviewsWithNitpick(t *testing.T) string {
	t.Helper()
	payload := []map[string]any{
		{
			"id": 901,
			"body": "<details>\n<summary>1 Nitpick comment</summary>\n<blockquote>\n<details>\n" +
				"<summary>internal/foo.go (1)</summary>\n<blockquote>\n" +
				"`12`: **Prefer wrapped errors**\nUse `%w` when returning the underlying error.\n" +
				"</blockquote>\n</details>\n</blockquote>\n</details>",
			"commit_id":    "head-sha",
			"state":        "COMMENTED",
			"submitted_at": "2026-07-07T12:00:00Z",
			"user": map[string]string{
				"login": codeRabbitBotLogin,
			},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal(codeRabbitReviewsWithNitpick) error = %v", err)
	}
	return string(encoded)
}

func codeRabbitStatuses(state string, description string) string {
	return fmt.Sprintf(
		`[{"state":%q,"description":%q,"context":"CodeRabbit","updated_at":"2026-07-07T12:00:00Z"}]`,
		state,
		description,
	)
}

func codeRabbitReviews(id string, commitID string, state string) string {
	return fmt.Sprintf(
		`[{"id":%s,"body":"review body","commit_id":%q,"state":%q,"submitted_at":"2026-07-07T12:00:00Z","user":{"login":"coderabbitai[bot]"}}]`,
		id,
		commitID,
		state,
	)
}

func decodeCodeRabbitWatchPayload(t *testing.T, raw json.RawMessage) codeRabbitWatchPayload {
	t.Helper()
	var payload codeRabbitWatchPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("Unmarshal(watch payload) error = %v; raw = %s", err, string(raw))
	}
	return payload
}
