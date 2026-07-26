package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"
	"net/url"
	"os"

	"path/filepath"

	"sort"
	"strings"

	aghcontract "github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/store"

	"github.com/compozy/agh/internal/transcript"
)

// CaptureSessionTranscript stores the session transcript artifact.
func (h *RuntimeHarness) CaptureSessionTranscript(ctx context.Context, sessionID string) error {
	response, err := h.SessionTranscript(ctx, sessionID)
	if err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindTranscript, transcript.MessagesFromEntries(response.Entries))
}

// CaptureSessionEvents stores the session-events artifact.
func (h *RuntimeHarness) CaptureSessionEvents(ctx context.Context, sessionID string) error {
	response, err := h.SessionEvents(ctx, sessionID)
	if err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindEvents, response.Events)
}

// CaptureSessionSandbox stores session sandbox metadata.
func (h *RuntimeHarness) CaptureSessionSandbox(ctx context.Context, sessionID string) error {
	artifact, err := h.SessionSandboxArtifact(ctx, sessionID)
	if err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindSessionSandbox, artifact)
}

// SessionSandboxArtifact reads the public session payload and, when present,
// the persisted session metadata for one runtime session.
func (h *RuntimeHarness) SessionSandboxArtifact(
	ctx context.Context,
	sessionID string,
) (SessionSandboxArtifact, error) {
	session, err := h.GetSession(ctx, sessionID)
	if err != nil {
		return SessionSandboxArtifact{}, err
	}
	artifact := SessionSandboxArtifact{
		SessionID:    session.ID,
		SessionState: string(session.State),
		StopReason:   session.StopReason,
		StopDetail:   session.StopDetail,
		API:          session.Sandbox,
	}

	metaPath := store.SessionMetaFile(filepath.Join(h.HomePaths.SessionsDir, strings.TrimSpace(sessionID)))
	meta, err := store.ReadSessionMeta(metaPath)
	switch {
	case err == nil:
		artifact.Persisted = meta.Sandbox
	case errors.Is(err, os.ErrNotExist):
		// Keep the public-surface artifact even when no persisted meta exists yet.
	default:
		return SessionSandboxArtifact{}, fmt.Errorf("read session meta %q: %w", metaPath, err)
	}
	return artifact, nil
}

// CaptureNetworkMessages stores the current network message projection for one channel.
func (h *RuntimeHarness) CaptureNetworkMessages(ctx context.Context, channel string) error {
	messages, err := h.NetworkChannelMessages(ctx, channel)
	if err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindNetworkMessages, messages)
}

// CaptureNetworkThreads stores public-thread summaries for one channel.
func (h *RuntimeHarness) CaptureNetworkThreads(ctx context.Context, channel string) error {
	threads, err := h.NetworkThreads(ctx, channel)
	if err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindNetworkThreads, threads)
}

// CaptureNetworkDirectRooms stores direct-room summaries for one channel.
func (h *RuntimeHarness) CaptureNetworkDirectRooms(ctx context.Context, channel string) error {
	directs, err := h.NetworkDirectRooms(ctx, channel)
	if err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindNetworkDirectRooms, directs)
}

// CaptureNetworkWork stores unique work rows referenced by the channel's thread and direct messages.
func (h *RuntimeHarness) CaptureNetworkWork(ctx context.Context, channel string) error {
	workIDs, err := h.networkWorkIDs(ctx, channel)
	if err != nil {
		return err
	}
	work := make([]aghcontract.NetworkWorkPayload, 0, len(workIDs))
	for _, workID := range workIDs {
		item, err := h.NetworkWork(ctx, workID)
		if err != nil {
			return err
		}
		work = append(work, item)
	}
	return h.Artifacts.CaptureJSON(ArtifactKindNetworkWork, work)
}

func (h *RuntimeHarness) networkWorkIDs(ctx context.Context, channel string) ([]string, error) {
	seen := make(map[string]struct{})
	add := func(messages []aghcontract.NetworkConversationMessagePayload) {
		for _, message := range messages {
			workID := strings.TrimSpace(message.WorkID)
			if workID != "" {
				seen[workID] = struct{}{}
			}
		}
	}

	threadMessages, err := h.NetworkChannelMessages(ctx, channel)
	if err != nil {
		return nil, err
	}
	add(threadMessages)

	directs, err := h.NetworkDirectRooms(ctx, channel)
	if err != nil {
		return nil, err
	}
	for _, direct := range directs {
		messages, err := h.NetworkDirectRoomMessages(ctx, channel, direct.DirectID)
		if err != nil {
			return nil, err
		}
		add(messages)
	}

	workIDs := make([]string, 0, len(seen))
	for workID := range seen {
		workIDs = append(workIDs, workID)
	}
	sort.Strings(workIDs)
	return workIDs, nil
}

// CaptureNetworkAudit stores the raw network audit sink when present.
func (h *RuntimeHarness) CaptureNetworkAudit() error {
	entries, err := h.NetworkAuditSnapshot()
	if err != nil {
		return err
	}
	if entries == nil {
		return nil
	}
	return h.Artifacts.CaptureJSON(ArtifactKindNetworkAudit, entries)
}

// CaptureNetworkArtifacts stores the stable message and audit snapshots for one scenario channel.
func (h *RuntimeHarness) CaptureNetworkArtifacts(ctx context.Context, channel string) error {
	if err := h.CaptureNetworkThreads(ctx, channel); err != nil {
		return err
	}
	if err := h.CaptureNetworkDirectRooms(ctx, channel); err != nil {
		return err
	}
	if err := h.CaptureNetworkMessages(ctx, channel); err != nil {
		return err
	}
	if err := h.CaptureNetworkWork(ctx, channel); err != nil {
		return err
	}
	return h.CaptureNetworkAudit()
}

// CaptureAutomationRuns stores the current automation run projection.
func (h *RuntimeHarness) CaptureAutomationRuns(ctx context.Context, query url.Values) error {
	var response aghcontract.RunsResponse
	if err := h.UDSJSON(ctx, http.MethodGet, "/api/automation/runs"+encodeQuery(query), nil, &response); err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindAutomationRuns, response.Runs)
}

// CaptureTasks stores the current task projection.
func (h *RuntimeHarness) CaptureTasks(ctx context.Context, query url.Values) error {
	var response aghcontract.TasksResponse
	if err := h.UDSJSON(ctx, http.MethodGet, "/api/tasks"+encodeQuery(query), nil, &response); err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindTasks, response.Tasks)
}

// CaptureTaskRuns stores the task-run projection for one task.
func (h *RuntimeHarness) CaptureTaskRuns(
	ctx context.Context,
	taskID string,
	query url.Values,
) error {
	var response aghcontract.TaskRunsResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodGet,
		"/api/tasks/"+taskID+"/runs"+encodeQuery(query),
		nil,
		&response,
	); err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindTaskRuns, response.Runs)
}

// CaptureBridgeHealth stores one bridge health-stream snapshot for the given bridge IDs.
func (h *RuntimeHarness) CaptureBridgeHealth(ctx context.Context, bridgeIDs ...string) (err error) {
	trimmed := make([]string, 0, len(bridgeIDs))
	for _, id := range bridgeIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		trimmed = append(trimmed, id)
	}
	if len(trimmed) == 0 {
		return errors.New("bridge health capture requires at least one bridge id")
	}

	query := url.Values{}
	query.Set("bridge_ids", strings.Join(trimmed, ","))
	response, err := doRequest(
		ctx,
		h.UDSClient,
		h.UDSURL("/api/bridges/health/stream")+"?"+query.Encode(),
		http.MethodGet,
		nil,
	)
	if err != nil {
		return err
	}
	defer mergeHTTPResponseCloseError(
		&err,
		response,
		http.MethodGet,
		h.UDSURL("/api/bridges/health/stream")+"?"+query.Encode(),
	)

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return fmt.Errorf("read bridge health failure response: %w", readErr)
		}
		return fmt.Errorf("bridge health status %d: %s", response.StatusCode, bytes.TrimSpace(payload))
	}

	records, err := readSSERecords(response.Body, 1)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return errors.New("bridge health stream returned no snapshot")
	}

	var snapshot aghcontract.BridgeHealthStreamPayload
	if err := json.Unmarshal(records[0].Data, &snapshot); err != nil {
		return fmt.Errorf("decode bridge health snapshot: %w", err)
	}
	return h.Artifacts.CaptureJSON(ArtifactKindBridgeHealth, snapshot)
}

// CaptureProviderCallsFile stores provider call markers or logs as a raw artifact.
func (h *RuntimeHarness) CaptureProviderCallsFile(path string, mediaType string) error {
	return h.Artifacts.CaptureFile(ArtifactKindProviderCalls, path, mediaType)
}

// CaptureProviderCallsJSON stores provider call diagnostics as JSON.
func (h *RuntimeHarness) CaptureProviderCallsJSON(value any) error {
	return h.Artifacts.CaptureJSON(ArtifactKindProviderCalls, value)
}

// CaptureToolHostDiagnosticsJSON stores tool-host diagnostics separately from
// provider or mock-agent artifacts so combined-flow runs can retain both.
func (h *RuntimeHarness) CaptureToolHostDiagnosticsJSON(value ToolHostDiagnosticsArtifact) error {
	return h.Artifacts.CaptureJSON(ArtifactKindToolHostDiagnostics, value)
}

// CaptureCombinedFlowJSON stores a cross-domain scenario summary alongside the
// domain-specific artifacts captured by the test.
func (h *RuntimeHarness) CaptureCombinedFlowJSON(value CombinedFlowArtifact) error {
	return h.Artifacts.CaptureJSON(ArtifactKindCombinedFlow, value)
}

// CaptureBrowserTraceFile stores the Playwright trace archive for one scenario.
func (h *RuntimeHarness) CaptureBrowserTraceFile(path string) error {
	return h.Artifacts.CaptureFile(ArtifactKindBrowserTrace, path, "application/zip")
}

// CaptureBrowserScreenshots stores one or more screenshot files.
func (h *RuntimeHarness) CaptureBrowserScreenshots(paths []string) error {
	return h.Artifacts.CaptureFiles(ArtifactKindBrowserScreenshots, paths, "image/png")
}

// CaptureBrowserConsoleJSON stores browser console diagnostics.
func (h *RuntimeHarness) CaptureBrowserConsoleJSON(value any) error {
	return h.Artifacts.CaptureJSON(ArtifactKindBrowserConsole, value)
}

// CaptureBrowserNetworkJSON stores browser network diagnostics.
func (h *RuntimeHarness) CaptureBrowserNetworkJSON(value any) error {
	return h.Artifacts.CaptureJSON(ArtifactKindBrowserNetwork, value)
}

// CaptureTransportOutput stores one transport result inside the shared harness artifact root.
func (h *RuntimeHarness) CaptureTransportOutput(
	name string,
	artifact TransportOutputArtifact,
) (string, error) {
	if h == nil {
		return "", errors.New("runtime harness is required")
	}
	if h.Artifacts == nil {
		return "", errors.New("runtime harness artifacts are required")
	}
	artifact.Name = defaultString(artifact.Name, name)
	path, err := h.Artifacts.CaptureNamedJSON(ArtifactKindTransportOutputs, name, artifact)
	if err != nil {
		return "", err
	}
	if _, err := h.WriteRuntimeManifest(); err != nil {
		return "", err
	}
	return path, nil
}

// CaptureCLIOutput stores one CLI command result in the shared transport-output artifact directory.
func (h *RuntimeHarness) CaptureCLIOutput(
	name string,
	args []string,
	stdout string,
	stderr string,
	commandErr error,
) (string, error) {
	artifact := TransportOutputArtifact{
		Name:      strings.TrimSpace(name),
		Transport: "cli",
		Command:   append([]string(nil), args...),
		Stdout:    stdout,
		Stderr:    stderr,
	}
	if commandErr != nil {
		artifact.Error = commandErr.Error()
	}
	return h.CaptureTransportOutput(name, artifact)
}

// Run executes one CLI command against the isolated daemon runtime.
func (c *CLIClient) Run(ctx context.Context, args ...string) (string, string, error) {
	return c.RunInDir(ctx, "", args...)
}
