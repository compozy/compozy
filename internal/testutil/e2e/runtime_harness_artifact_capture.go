package e2e

import (
	"context"
	"errors"

	"net/http"
	"net/url"

	"strings"

	compozycontract "github.com/compozy/compozy/internal/api/contract"

	"github.com/compozy/compozy/internal/transcript"
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

// CaptureAutomationRuns stores the current automation run projection.
func (h *RuntimeHarness) CaptureAutomationRuns(ctx context.Context, query url.Values) error {
	var response compozycontract.RunsResponse
	if err := h.UDSJSON(ctx, http.MethodGet, "/api/automation/runs"+encodeQuery(query), nil, &response); err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindAutomationRuns, response.Runs)
}

// CaptureTasks stores the current task projection.
func (h *RuntimeHarness) CaptureTasks(ctx context.Context, query url.Values) error {
	var response compozycontract.TasksResponse
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
	var response compozycontract.TaskRunsResponse
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
