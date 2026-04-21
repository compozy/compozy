package cli

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/daemon"
)

func TestDaemonPublicSnapshotAndStreamMatchAcrossHTTPAndUDSForTempWorkspaceRun(t *testing.T) {
	homeDir := newShortCLITestHomeDir(t)
	t.Setenv("HOME", homeDir)
	configureCLITestDaemonHTTPPort(t)

	paths := mustCLITestHomePaths(t)
	workspaceRoot := t.TempDir()
	writeCLINodeWorkflowFixture(t, workspaceRoot, "node-health")
	writeCLIWorkspaceConfig(t, workspaceRoot, "")

	t.Cleanup(func() {
		_, _, _ = runCLICommand(t, workspaceRoot, "daemon", "stop", "--force", "--format", "json")
		waitForCLITestDaemonState(t, paths, daemon.ReadyStateStopped)
	})

	startStdout, startStderr, exitCode := runCLICommand(t, workspaceRoot, "daemon", "start", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("execute daemon start: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, startStdout, startStderr)
	}
	var startPayload struct {
		State  string `json:"state"`
		Health struct {
			Ready bool `json:"ready"`
		} `json:"health"`
		Daemon struct {
			HTTPPort int `json:"http_port"`
		} `json:"daemon"`
	}
	if err := json.Unmarshal([]byte(startStdout), &startPayload); err != nil {
		t.Fatalf("decode daemon start payload: %v\nstdout:\n%s", err, startStdout)
	}
	if startPayload.State != string(daemon.ReadyStateReady) || !startPayload.Health.Ready ||
		startPayload.Daemon.HTTPPort <= 0 {
		t.Fatalf("unexpected daemon start payload: %#v", startPayload)
	}

	status, err := daemon.QueryStatus(context.Background(), paths, daemon.ProbeOptions{})
	if err != nil {
		t.Fatalf("QueryStatus() error = %v", err)
	}
	if status.Info == nil {
		t.Fatal("status.Info = nil, want running daemon info")
	}

	httpClient, err := apiclient.New(apiclient.Target{HTTPPort: status.Info.HTTPPort})
	if err != nil {
		t.Fatalf("apiclient.New(http) error = %v", err)
	}
	udsClient, err := apiclient.New(apiclient.Target{SocketPath: status.Info.SocketPath})
	if err != nil {
		t.Fatalf("apiclient.New(uds) error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	httpStatus, err := httpClient.DaemonStatus(ctx)
	if err != nil {
		t.Fatalf("http DaemonStatus() error = %v", err)
	}
	udsStatus, err := udsClient.DaemonStatus(ctx)
	if err != nil {
		t.Fatalf("uds DaemonStatus() error = %v", err)
	}
	if !reflect.DeepEqual(httpStatus, udsStatus) {
		t.Fatalf("daemon status mismatch:\nhttp=%#v\nuds=%#v", httpStatus, udsStatus)
	}

	httpHealth, err := httpClient.Health(ctx)
	if err != nil {
		t.Fatalf("http Health() error = %v", err)
	}
	udsHealth, err := udsClient.Health(ctx)
	if err != nil {
		t.Fatalf("uds Health() error = %v", err)
	}
	assertDaemonHealthParity(t, httpHealth, udsHealth)
	if !httpHealth.Ready {
		t.Fatalf("expected ready health payload, got %#v", httpHealth)
	}

	syncStdout, syncStderr, exitCode := runCLICommand(
		t,
		workspaceRoot,
		"sync",
		"--name",
		"node-health",
		"--format",
		"json",
	)
	if exitCode != 0 {
		t.Fatalf("execute sync: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, syncStdout, syncStderr)
	}
	var syncPayload struct {
		WorkflowSlug      string `json:"workflow_slug"`
		WorkflowsScanned  int    `json:"workflows_scanned"`
		TaskItemsUpserted int    `json:"task_items_upserted"`
	}
	if err := json.Unmarshal([]byte(syncStdout), &syncPayload); err != nil {
		t.Fatalf("decode sync payload: %v\nstdout:\n%s", err, syncStdout)
	}
	if syncPayload.WorkflowSlug != "node-health" || syncPayload.WorkflowsScanned != 1 ||
		syncPayload.TaskItemsUpserted != 1 {
		t.Fatalf("unexpected sync payload: %#v\nstderr:\n%s", syncPayload, syncStderr)
	}

	taskStdout, taskStderr, exitCode := runCLICommand(
		t,
		workspaceRoot,
		"tasks",
		"run",
		"node-health",
		"--dry-run",
		"--stream",
	)
	if exitCode != 0 {
		t.Fatalf("execute tasks run dry-run: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, taskStdout, taskStderr)
	}
	if !strings.Contains(taskStderr, "preflight=ok") {
		t.Fatalf("expected task preflight success on stderr, got %q", taskStderr)
	}
	runID := parseStartedTaskRunID(t, taskStdout)

	httpSnapshot, err := httpClient.GetRunSnapshot(ctx, runID)
	if err != nil {
		t.Fatalf("http GetRunSnapshot() error = %v", err)
	}
	udsSnapshot, err := udsClient.GetRunSnapshot(ctx, runID)
	if err != nil {
		t.Fatalf("uds GetRunSnapshot() error = %v", err)
	}
	if !reflect.DeepEqual(httpSnapshot, udsSnapshot) {
		t.Fatalf("run snapshot mismatch:\nhttp=%#v\nuds=%#v", httpSnapshot, udsSnapshot)
	}
	if httpSnapshot.Run.RunID != runID || httpSnapshot.Run.Status != "completed" {
		t.Fatalf("unexpected terminal snapshot: %#v", httpSnapshot.Run)
	}

	httpStream, err := httpClient.OpenRunStream(ctx, runID, apicore.StreamCursor{})
	if err != nil {
		t.Fatalf("http OpenRunStream() error = %v", err)
	}
	httpItems := collectRunStreamSummaries(t, httpStream)

	udsStream, err := udsClient.OpenRunStream(ctx, runID, apicore.StreamCursor{})
	if err != nil {
		t.Fatalf("uds OpenRunStream() error = %v", err)
	}
	udsItems := collectRunStreamSummaries(t, udsStream)

	if !reflect.DeepEqual(httpItems, udsItems) {
		t.Fatalf("run stream mismatch:\nhttp=%#v\nuds=%#v", httpItems, udsItems)
	}
	if len(httpItems) == 0 || httpItems[len(httpItems)-1].Kind != "run.completed" {
		t.Fatalf("expected terminal stream summary, got %#v", httpItems)
	}
}

type runStreamSummary struct {
	Kind      string
	Seq       uint64
	Heartbeat string
	Overflow  string
	Reason    string
}

func parseStartedTaskRunID(t *testing.T, output string) string {
	t.Helper()

	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "task run started: ") {
			continue
		}
		rest := strings.TrimPrefix(line, "task run started: ")
		idx := strings.Index(rest, " (mode=")
		if idx > 0 {
			return rest[:idx]
		}
	}

	t.Fatalf("could not parse task run id from output:\n%s", output)
	return ""
}

func collectRunStreamSummaries(t *testing.T, stream apiclient.RunStream) []runStreamSummary {
	t.Helper()

	t.Cleanup(func() {
		_ = stream.Close()
	})

	items := stream.Items()
	errs := stream.Errors()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	var summaries []runStreamSummary
	for items != nil || errs != nil {
		select {
		case item, ok := <-items:
			if !ok {
				items = nil
				continue
			}
			switch {
			case item.Event != nil:
				summary := runStreamSummary{
					Kind: string(item.Event.Kind),
					Seq:  item.Event.Seq,
				}
				summaries = append(summaries, summary)
				if summary.Kind == "run.completed" || summary.Kind == "run.failed" || summary.Kind == "run.cancelled" ||
					summary.Kind == "run.crashed" {
					return summaries
				}
			case item.Heartbeat != nil:
				summaries = append(summaries, runStreamSummary{
					Kind:      "heartbeat",
					Heartbeat: apicore.FormatCursor(item.Heartbeat.Timestamp, item.Heartbeat.Cursor.Sequence),
				})
			case item.Overflow != nil:
				summaries = append(summaries, runStreamSummary{
					Kind:     "overflow",
					Overflow: apicore.FormatCursor(item.Overflow.Timestamp, item.Overflow.Cursor.Sequence),
					Reason:   item.Overflow.Reason,
				})
			default:
				t.Fatalf("unexpected empty stream item: %#v", item)
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(2 * time.Second)
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			if err != nil {
				t.Fatalf("run stream error: %v", err)
			}
		case <-timer.C:
			t.Fatal("timed out waiting for daemon run stream to finish")
		}
	}

	return summaries
}

func assertDaemonHealthParity(t *testing.T, httpHealth, udsHealth apicore.DaemonHealth) {
	t.Helper()

	if httpHealth.Ready != udsHealth.Ready ||
		httpHealth.Degraded != udsHealth.Degraded ||
		!httpHealth.StartedAt.Equal(udsHealth.StartedAt) ||
		httpHealth.ActiveRunCount != udsHealth.ActiveRunCount ||
		httpHealth.WorkspaceCount != udsHealth.WorkspaceCount ||
		httpHealth.IntegrityIssueCount != udsHealth.IntegrityIssueCount ||
		!reflect.DeepEqual(httpHealth.ActiveRunsByMode, udsHealth.ActiveRunsByMode) ||
		!reflect.DeepEqual(httpHealth.Reconcile, udsHealth.Reconcile) ||
		!reflect.DeepEqual(httpHealth.Details, udsHealth.Details) {
		t.Fatalf("daemon health mismatch:\nhttp=%#v\nuds=%#v", httpHealth, udsHealth)
	}

	if diff := httpHealth.UptimeSeconds - udsHealth.UptimeSeconds; diff < -1 || diff > 1 {
		t.Fatalf("daemon health uptime mismatch: http=%d uds=%d", httpHealth.UptimeSeconds, udsHealth.UptimeSeconds)
	}
	if httpHealth.Databases.GlobalBytes < 0 || udsHealth.Databases.GlobalBytes < 0 {
		t.Fatalf(
			"daemon global db bytes must be non-negative: http=%d uds=%d",
			httpHealth.Databases.GlobalBytes,
			udsHealth.Databases.GlobalBytes,
		)
	}
	if httpHealth.Databases.RunDBBytes < 0 || udsHealth.Databases.RunDBBytes < 0 {
		t.Fatalf(
			"daemon run db bytes must be non-negative: http=%d uds=%d",
			httpHealth.Databases.RunDBBytes,
			udsHealth.Databases.RunDBBytes,
		)
	}
}
