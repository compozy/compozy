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

	"strings"
	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	sessionpkg "github.com/compozy/agh/internal/session"
)

// RuntimeManifestPath returns the stable runtime-manifest path under the harness artifact root.
func (h *RuntimeHarness) RuntimeManifestPath() string {
	if h == nil || h.Artifacts == nil {
		return ""
	}
	return filepath.Join(h.Artifacts.RootDir(), runtimeManifestName)
}

// RuntimeManifest returns the current runtime-manifest snapshot without writing it.
func (h *RuntimeHarness) RuntimeManifest() (RuntimeArtifactManifest, error) {
	if h == nil {
		return RuntimeArtifactManifest{}, errors.New("runtime harness is required")
	}
	if h.Artifacts == nil {
		return RuntimeArtifactManifest{}, errors.New("runtime harness artifacts are required")
	}

	runDirectories, err := runtimeRunDirectories(h.HomePaths.SessionsDir)
	if err != nil {
		return RuntimeArtifactManifest{}, err
	}
	cliBinary := ""
	cliWorkdir := ""
	if h.CLI != nil {
		cliBinary = strings.TrimSpace(h.CLI.binaryPath)
		cliWorkdir = strings.TrimSpace(h.CLI.workdir)
	}

	return RuntimeArtifactManifest{
		Version:       1,
		WorkspaceRoot: strings.TrimSpace(h.WorkspaceRoot),
		Home: RuntimeHomeArtifact{
			HomeDir:          strings.TrimSpace(h.HomePaths.HomeDir),
			ConfigFile:       strings.TrimSpace(h.HomePaths.ConfigFile),
			DatabaseFile:     strings.TrimSpace(h.HomePaths.DatabaseFile),
			DaemonSocket:     strings.TrimSpace(h.HomePaths.DaemonSocket),
			DaemonInfo:       strings.TrimSpace(h.HomePaths.DaemonInfo),
			LogsDir:          strings.TrimSpace(h.HomePaths.LogsDir),
			NetworkAuditFile: strings.TrimSpace(h.HomePaths.NetworkAuditFile),
		},
		Logs: RuntimeLogArtifact{
			DaemonLogFile:  strings.TrimSpace(h.HomePaths.LogFile),
			ProcessLogFile: strings.TrimSpace(h.processLogPath),
		},
		Runs: RuntimeRunArtifact{
			RootDir:     strings.TrimSpace(h.HomePaths.SessionsDir),
			Directories: runDirectories,
		},
		Transport: RuntimeTransportArtifact{
			HTTPBaseURL: strings.TrimSpace(h.HTTPBaseURL),
			HTTPHost:    strings.TrimSpace(h.Config.HTTP.Host),
			HTTPPort:    h.Config.HTTP.Port,
			UDSBaseURL:  strings.TrimSpace(h.UDSBaseURL),
			SocketPath:  strings.TrimSpace(h.Config.Daemon.Socket),
			CLIBinary:   cliBinary,
			CLIWorkdir:  cliWorkdir,
		},
		ArtifactRootDir:      strings.TrimSpace(h.Artifacts.RootDir()),
		ArtifactManifestPath: strings.TrimSpace(h.Artifacts.ManifestPath()),
		CapturedArtifacts:    h.Artifacts.Manifest(),
	}, nil
}

// WriteRuntimeManifest persists the current runtime-manifest snapshot.
func (h *RuntimeHarness) WriteRuntimeManifest() (RuntimeArtifactManifest, error) {
	if h == nil || h.Artifacts == nil {
		return RuntimeArtifactManifest{}, errors.New("runtime harness artifacts are required")
	}
	if _, err := h.Artifacts.WriteManifest(); err != nil {
		return RuntimeArtifactManifest{}, err
	}
	manifest, err := h.RuntimeManifest()
	if err != nil {
		return RuntimeArtifactManifest{}, err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return RuntimeArtifactManifest{}, fmt.Errorf("marshal runtime manifest: %w", err)
	}
	data = append(data, '\n')

	path := h.RuntimeManifestPath()
	if strings.TrimSpace(path) == "" {
		return RuntimeArtifactManifest{}, errors.New("runtime harness artifact root is required")
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return RuntimeArtifactManifest{}, fmt.Errorf("write runtime manifest %q: %w", path, err)
	}
	return manifest, nil
}

// HTTPURL returns one absolute HTTP URL under the public daemon surface.
func (h *RuntimeHarness) HTTPURL(path string) string {
	return h.HTTPBaseURL + ensureLeadingSlash(path)
}

// UDSURL returns one absolute UDS-backed URL under the operator daemon surface.
func (h *RuntimeHarness) UDSURL(path string) string {
	return h.UDSBaseURL + ensureLeadingSlash(path)
}

// HTTPJSON performs a JSON request against the daemon HTTP API.
func (h *RuntimeHarness) HTTPJSON(
	ctx context.Context,
	method string,
	path string,
	body any,
	dest any,
) error {
	return doJSONRequest(ctx, h.HTTPClient, h.HTTPURL(path), method, body, dest)
}

// UDSJSON performs a JSON request against the daemon UDS API.
func (h *RuntimeHarness) UDSJSON(
	ctx context.Context,
	method string,
	path string,
	body any,
	dest any,
) error {
	return doJSONRequest(ctx, h.UDSClient, h.UDSURL(path), method, body, dest)
}

// ResolveWorkspace resolves a workspace through the real daemon API.
func (h *RuntimeHarness) ResolveWorkspace(
	ctx context.Context,
	root string,
) (aghcontract.WorkspacePayload, error) {
	var response aghcontract.WorkspaceResponse
	err := h.UDSJSON(
		ctx,
		http.MethodPost,
		"/api/workspaces/resolve",
		aghcontract.ResolveWorkspaceRequest{Path: root},
		&response,
	)
	if err != nil {
		return aghcontract.WorkspacePayload{}, err
	}
	h.WorkspaceID = response.Workspace.ID
	return response.Workspace, nil
}

// GetWorkspace fetches one workspace through the daemon operator surface.
func (h *RuntimeHarness) GetWorkspace(
	ctx context.Context,
	workspaceID string,
) (aghcontract.WorkspacePayload, error) {
	var response aghcontract.WorkspaceResponse
	if err := h.UDSJSON(ctx, http.MethodGet, "/api/workspaces/"+workspaceID, nil, &response); err != nil {
		return aghcontract.WorkspacePayload{}, err
	}
	return response.Workspace, nil
}

func (h *RuntimeHarness) workspaceScopedAPIPath(workspaceID string, suffix string) (string, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(h.WorkspaceID)
	}
	if workspaceID == "" {
		return "", errors.New("runtime harness workspace ID is required")
	}
	if !strings.HasPrefix(suffix, "/") {
		suffix = "/" + suffix
	}
	return "/api/workspaces/" + url.PathEscape(workspaceID) + suffix, nil
}

func (h *RuntimeHarness) sessionScopedAPIPath(sessionID string, suffix string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", errors.New("session ID is required")
	}
	return h.workspaceScopedAPIPath("", "/sessions/"+url.PathEscape(sessionID)+suffix)
}

func (h *RuntimeHarness) networkScopedAPIPath(workspaceID string, suffix string) (string, error) {
	return h.workspaceScopedAPIPath(workspaceID, "/network"+suffix)
}

// CreateSession creates one session through the operator surface.
func (h *RuntimeHarness) CreateSession(
	ctx context.Context,
	request aghcontract.CreateSessionRequest,
) (aghcontract.SessionPayload, error) {
	var response aghcontract.SessionResponse
	if err := h.UDSJSON(ctx, http.MethodPost, "/api/sessions", request, &response); err != nil {
		return aghcontract.SessionPayload{}, err
	}
	return response.Session, nil
}

// GetSession fetches one session detail through the operator surface.
func (h *RuntimeHarness) GetSession(
	ctx context.Context,
	sessionID string,
) (aghcontract.SessionPayload, error) {
	var response aghcontract.SessionResponse
	path, err := h.sessionScopedAPIPath(sessionID, "")
	if err != nil {
		return aghcontract.SessionPayload{}, err
	}
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return aghcontract.SessionPayload{}, err
	}
	return response.Session, nil
}

// WaitForSessionActive waits for asynchronous session startup to become prompt-ready.
func (h *RuntimeHarness) WaitForSessionActive(
	ctx context.Context,
	sessionID string,
) (aghcontract.SessionPayload, error) {
	if ctx == nil {
		return aghcontract.SessionPayload{}, errors.New("runtime harness session wait context is required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return aghcontract.SessionPayload{}, errors.New("runtime harness session wait ID is required")
	}

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	var (
		current aghcontract.SessionPayload
		lastErr error
	)
	for {
		resolved, err := h.GetSession(ctx, sessionID)
		if err == nil {
			current = resolved
			switch current.State {
			case sessionpkg.StateActive:
				return current, nil
			case sessionpkg.StateStopped:
				return current, fmt.Errorf(
					"runtime harness session %q startup stopped: failure=%#v",
					sessionID,
					current.Failure,
				)
			}
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return current, errors.Join(
				fmt.Errorf("runtime harness wait for session %q active: %w; last=%#v", sessionID, ctx.Err(), current),
				lastErr,
			)
		case <-ticker.C:
		}
	}
}

// StopSession stops one session through the operator surface.
func (h *RuntimeHarness) StopSession(ctx context.Context, sessionID string) (err error) {
	path, err := h.sessionScopedAPIPath(sessionID, "/stop")
	if err != nil {
		return err
	}
	response, err := doRequest(
		ctx,
		h.UDSClient,
		h.UDSURL(path),
		http.MethodPost,
		nil,
	)
	if err != nil {
		return err
	}
	defer mergeHTTPResponseCloseError(&err, response, http.MethodPost, h.UDSURL(path))

	if response.StatusCode != http.StatusNoContent {
		payload, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return fmt.Errorf("read stop session failure response: %w", readErr)
		}
		return fmt.Errorf("stop session status %d: %s", response.StatusCode, bytes.TrimSpace(payload))
	}
	return nil
}

// ResumeSession attaches to one resumable live session through the operator surface.
func (h *RuntimeHarness) ResumeSession(
	ctx context.Context,
	sessionID string,
) (aghcontract.SessionPayload, error) {
	var response aghcontract.SessionAttachResponse
	path, err := h.sessionScopedAPIPath(sessionID, "/attach")
	if err != nil {
		return aghcontract.SessionPayload{}, err
	}
	if err := h.UDSJSON(
		ctx,
		http.MethodPost,
		path,
		nil,
		&response,
	); err != nil {
		return aghcontract.SessionPayload{}, err
	}
	return response.Session, nil
}

// PromptSession sends one prompt through the operator surface and drains the SSE stream.
func (h *RuntimeHarness) PromptSession(
	ctx context.Context,
	sessionID string,
	message string,
) ([]SSEEvent, error) {
	return h.PromptSessionWithEvents(ctx, sessionID, message, nil)
}

// PromptSessionWithEvents sends one prompt through the operator surface and lets
// callers react to streamed SSE records before the prompt completes.
func (h *RuntimeHarness) PromptSessionWithEvents(
	ctx context.Context,
	sessionID string,
	message string,
	onEvent func(SSEEvent) error,
) (_ []SSEEvent, err error) {
	body := map[string]string{runtimeHarnessMessageKey: message}
	path, err := h.sessionScopedAPIPath(sessionID, "/prompt")
	if err != nil {
		return nil, err
	}
	response, err := doRequest(ctx, h.UDSClient, h.UDSURL(path), http.MethodPost, body)
	if err != nil {
		return nil, err
	}
	defer mergeHTTPResponseCloseError(&err, response, http.MethodPost, h.UDSURL(path))

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read prompt failure response: %w", readErr)
		}
		return nil, fmt.Errorf("prompt session status %d: %s", response.StatusCode, bytes.TrimSpace(payload))
	}

	return readSSERecordsWithCallback(response.Body, 0, onEvent)
}

// PromptSessionUntil sends one prompt through the operator surface and returns
// as soon as the streamed SSE records satisfy predicate.
func (h *RuntimeHarness) PromptSessionUntil(
	ctx context.Context,
	sessionID string,
	message string,
	predicate func(SSEEvent) bool,
) (_ []SSEEvent, err error) {
	if err := validateSSEPredicate(predicate); err != nil {
		return nil, err
	}
	body := map[string]string{runtimeHarnessMessageKey: message}
	path, err := h.sessionScopedAPIPath(sessionID, "/prompt")
	if err != nil {
		return nil, err
	}
	response, err := doRequest(ctx, h.UDSClient, h.UDSURL(path), http.MethodPost, body)
	if err != nil {
		return nil, err
	}
	defer mergeHTTPResponseCloseError(&err, response, http.MethodPost, h.UDSURL(path))

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read prompt failure response: %w", readErr)
		}
		return nil, fmt.Errorf("prompt session status %d: %s", response.StatusCode, bytes.TrimSpace(payload))
	}

	return readSSERecordsUntil(response.Body, predicate)
}

// SessionTranscript fetches the persisted transcript for one session.
func (h *RuntimeHarness) SessionTranscript(
	ctx context.Context,
	sessionID string,
) (aghcontract.SessionTranscriptResponse, error) {
	var response aghcontract.SessionTranscriptResponse
	path, err := h.sessionScopedAPIPath(sessionID, "/transcript")
	if err != nil {
		return aghcontract.SessionTranscriptResponse{}, err
	}
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return aghcontract.SessionTranscriptResponse{}, err
	}
	return response, nil
}

// SessionEvents fetches persisted events for one session.
func (h *RuntimeHarness) SessionEvents(
	ctx context.Context,
	sessionID string,
) (aghcontract.SessionEventsResponse, error) {
	var response aghcontract.SessionEventsResponse
	path, err := h.sessionScopedAPIPath(sessionID, "/events")
	if err != nil {
		return aghcontract.SessionEventsResponse{}, err
	}
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return aghcontract.SessionEventsResponse{}, err
	}
	return response, nil
}
