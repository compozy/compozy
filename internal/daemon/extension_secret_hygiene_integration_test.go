//go:build integration && !windows

package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestExtensionSecretTransportAbsence(t *testing.T) {
	t.Run("Should keep extension secrets out of every transport", testExtensionSecretTransportAbsence)
}

func testExtensionSecretTransportAbsence(t *testing.T) {
	t.Parallel()

	const (
		runtimeSecret     = "extension-runtime-secret-7b9e52f0"
		publishCredential = "extension-publish-token-a4c831d2"
		workspaceID       = "workspace-secret-hygiene"
		extensionName     = "secret-hygiene"
	)
	secrets := []string{runtimeSecret, publishCredential}

	homePaths := testHomePaths(t)
	db := openDaemonTestGlobalDB(t)
	extensionRegistry := extensionpkg.NewRegistry(db.DB())
	workspaceRoot := t.TempDir()
	workspaceResolver := &daemonExtensionWorkspaceResolverStub{
		resolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{
				ID:      workspaceID,
				Name:    "secret-hygiene",
				RootDir: workspaceRoot,
			},
			WorkspaceID: workspaceID,
		},
	}
	secretResolver := nativeExtensionSecretResolver{values: map[string]string{
		"env:EXT_RUNTIME_SECRET": runtimeSecret,
		"env:GITHUB_TOKEN":       publishCredential,
	}}
	manager := extensionpkg.NewManager(
		extensionRegistry,
		extensionpkg.WithLogger(discardLogger()),
		extensionpkg.WithWorkspaceResolver(workspaceResolver),
		extensionpkg.WithSecretResolver(secretResolver),
	)
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Errorf("Manager.Stop() error = %v", err)
		}
	})

	service, ok := newDaemonExtensionService(
		extensionRegistry,
		manager,
		nil,
		nil,
		nil,
		nil,
		nil,
		homePaths,
		discardLogger(),
		time.Now,
		withDaemonExtensionMarketplace(
			compozyconfig.ExtensionsConfig{Trust: compozyconfig.ExtensionsTrustConfig{AllowUnverified: true}},
			nil,
		),
		withDaemonExtensionEventWriter(db),
	).(*daemonExtensionService)
	if !ok {
		t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
	}
	actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator",
		workspaceID,
		taskpkg.OriginKindHTTP,
		"extension secret hygiene",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}

	httpEngine := newExtensionSecretTransportEngine(service, actor, "http")
	udsEngine := newExtensionSecretTransportEngine(service, actor, "uds")
	fixtureDir := writeSecretExtensionFixture(t, t.TempDir(), extensionName, "1.0.0")
	installBody := mustSecretTransportJSON(t, contract.InstallExtensionRequest{
		Source:          contract.InstallExtensionSourceLocalPath,
		Ref:             fixtureDir,
		AllowUnverified: true,
	})
	installResponse := performSecretTransportRequest(
		t,
		httpEngine,
		http.MethodPost,
		"/extensions",
		installBody,
	)
	if installResponse.Code != http.StatusCreated {
		t.Fatalf(
			"HTTP install status = %d, want %d; body=%s",
			installResponse.Code,
			http.StatusCreated,
			installResponse.Body,
		)
	}
	assertSecretsAbsent(t, "HTTP install response", installResponse.Body.String(), secrets)

	origin := filepath.Join(workspaceRoot, "secret-extension")
	firstGeneration := writeSecretExtensionGeneration(t, origin, extensionName, "1.1.0")
	devResponse := performSecretTransportRequest(
		t,
		httpEngine,
		http.MethodPost,
		"/extensions/dev",
		mustSecretTransportJSON(t, contract.DevLinkExtensionRequest{
			OriginPath: origin, GenerationHash: firstGeneration,
		}),
	)
	if devResponse.Code != http.StatusCreated {
		t.Fatalf("HTTP dev status = %d, want %d; body=%s", devResponse.Code, http.StatusCreated, devResponse.Body)
	}
	assertSecretsAbsent(t, "HTTP dev response", devResponse.Body.String(), secrets)

	secondGeneration := writeSecretExtensionGeneration(t, origin, extensionName, "1.2.0")
	reloadResponse := performSecretTransportRequest(
		t,
		udsEngine,
		http.MethodPost,
		"/extensions/"+extensionName+"/reload",
		mustSecretTransportJSON(t, contract.ReloadExtensionRequest{GenerationHash: secondGeneration}),
	)
	if reloadResponse.Code != http.StatusOK {
		t.Fatalf("UDS reload status = %d, want %d; body=%s", reloadResponse.Code, http.StatusOK, reloadResponse.Body)
	}
	assertSecretsAbsent(t, "UDS reload response", reloadResponse.Body.String(), secrets)

	logs := waitForSecretExtensionLogs(t, service, extensionName, actor)
	logsJSON := mustSecretTransportJSON(t, logs)
	assertSecretsAbsent(t, "extension log ring", string(logsJSON), secrets)
	if !bytes.Contains(logsJSON, []byte("[REDACTED]")) || !bytes.Contains(logsJSON, []byte("safe=visible")) {
		t.Fatalf("extension log ring = %s, want redaction marker and safe field", logsJSON)
	}

	for transport, engine := range map[string]*gin.Engine{"HTTP": httpEngine, "UDS": udsEngine} {
		response := performSecretTransportRequest(
			t,
			engine,
			http.MethodGet,
			"/extensions/"+extensionName+"/logs",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("%s logs status = %d, want %d; body=%s", transport, response.Code, http.StatusOK, response.Body)
		}
		assertSecretsAbsent(t, transport+" logs response", response.Body.String(), secrets)
	}

	sseFrame := readSecretExtensionSSEFrame(t, httpEngine, extensionName)
	assertSecretsAbsent(t, "HTTP SSE stream", sseFrame, secrets)
	if !strings.Contains(sseFrame, "[REDACTED]") {
		t.Fatalf("HTTP SSE stream = %q, want redaction marker", sseFrame)
	}

	publishServer, publishCapture := newNativeExtensionPublishServer(t, publishCredential)
	t.Cleanup(publishServer.Close)
	deps := &daemonNativeToolsDeps{
		ExtensionRegistry: extensionRegistry,
		Extensions: func() core.ExtensionService {
			return service
		},
		ExtensionRuntime: func() extensionRuntime { return manager },
		ExtensionConfig: compozyconfig.ExtensionsConfig{Sources: compozyconfig.ExtensionsSourcesConfig{
			GitHub: compozyconfig.ExtensionsGitHubSourceConfig{BaseURL: publishServer.URL},
		}},
		ExtensionEvents:   db,
		ExtensionSecrets:  secretResolver,
		HomePaths:         homePaths,
		WorkspaceResolver: workspaceResolver,
	}
	nativeRegistry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())
	nativeScope := toolspkg.Scope{WorkspaceID: workspaceID, Operator: true}
	nativeLogs, err := nativeRegistry.Call(t.Context(), nativeScope, toolspkg.CallRequest{
		ToolID:      toolspkg.ToolIDExtensionsLogs,
		WorkspaceID: workspaceID,
		Input:       json.RawMessage(`{"name":"secret-hygiene","after":0}`),
	})
	if err != nil {
		t.Fatalf("Registry.Call(extensions_logs) error = %v", err)
	}
	assertSecretsAbsent(t, "native logs structured result", string(nativeLogs.Structured), secrets)
	assertSecretsAbsent(t, "native logs preview", nativeLogs.Preview, secrets)

	generationDir := filepath.Join(origin, "dist", "gen-"+secondGeneration)
	publishResult, err := nativeRegistry.Call(t.Context(), nativeScope, toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDExtensionsPublish,
		Input: json.RawMessage(fmt.Sprintf(
			`{"generation_dir":%q,"repository":"acme/native-dev","tag_name":"v0.2.0"}`,
			generationDir,
		)),
	})
	if err != nil {
		t.Fatalf("Registry.Call(extensions_publish) error = %v", err)
	}
	assertSecretsAbsent(t, "native publish structured result", string(publishResult.Structured), secrets)
	assertSecretsAbsent(t, "native publish preview", publishResult.Preview, secrets)
	publishCapture.requireComplete(t, publishCredential)

	failureServer := newSecretEchoingPublishFailureServer(t, secrets)
	t.Cleanup(failureServer.Close)
	deps.ExtensionConfig.Sources.GitHub.BaseURL = failureServer.URL
	failingNativeRegistry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())
	_, publishErr := failingNativeRegistry.Call(t.Context(), nativeScope, toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDExtensionsPublish,
		Input: json.RawMessage(fmt.Sprintf(
			`{"generation_dir":%q,"repository":"acme/native-dev","tag_name":"v0.2.0"}`,
			generationDir,
		)),
	})
	if publishErr == nil {
		t.Fatal("Registry.Call(extensions_publish failure) error = nil, want structured error")
	}
	structuredError := mustSecretTransportJSON(t, core.ErrorPayloadForError(publishErr))
	assertSecretsAbsent(t, "structured publish error", string(structuredError), secrets)

	events, err := db.ListEventSummaries(t.Context(), store.EventSummaryQuery{})
	if err != nil {
		t.Fatalf("ListEventSummaries() error = %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("persisted extension events = %d, want install/dev/reload", len(events))
	}
	assertSecretsAbsent(t, "persisted extension events", string(mustSecretTransportJSON(t, events)), secrets)
}

func newExtensionSecretTransportEngine(
	service core.ExtensionService,
	actor taskpkg.ActorContext,
	transport string,
) *gin.Engine {
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: transport,
		Extensions:    service,
		TaskActorContextResolver: func(*gin.Context, string) (taskpkg.ActorContext, error) {
			return actor, nil
		},
	})
	engine := gin.New()
	engine.POST("/extensions", handlers.InstallExtension)
	engine.POST("/extensions/dev", handlers.DevExtension)
	engine.POST("/extensions/:name/reload", handlers.ReloadDevExtension)
	engine.GET("/extensions/:name/logs", handlers.ExtensionLogs)
	return engine
}

func writeSecretExtensionFixture(t *testing.T, root, name, version string) string {
	t.Helper()
	dir := filepath.Join(root, name+"-"+version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
	}
	manifest := daemonTestExtensionManifest(name, daemonTestExtensionOptions{
		runtimeCommand: daemonExtensionHelperCommand(t),
		runtimeArgs:    daemonExtensionHelperArgs(),
		runtimeEnv:     daemonExtensionHelperScenarioEnv("secret_hygiene", ""),
		runtimeSecretEnv: map[string]string{
			"BOUND_PROVIDER_TOKEN": "env:GITHUB_TOKEN",
			"BOUND_SECRET":         "env:EXT_RUNTIME_SECRET",
		},
	})
	manifest = strings.Replace(manifest, `version = "0.2.1"`, fmt.Sprintf("version = %q", version), 1)
	if err := os.WriteFile(filepath.Join(dir, "extension.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}
	return dir
}

func writeSecretExtensionGeneration(t *testing.T, origin, name, version string) string {
	t.Helper()
	fixture := writeSecretExtensionFixture(t, t.TempDir(), name, version)
	manifest, err := os.ReadFile(filepath.Join(fixture, "extension.toml"))
	if err != nil {
		t.Fatalf("os.ReadFile(extension.toml) error = %v", err)
	}
	dist := filepath.Join(origin, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", dist, err)
	}
	staging, err := os.MkdirTemp(dist, ".secret-generation-")
	if err != nil {
		t.Fatalf("os.MkdirTemp(%q) error = %v", dist, err)
	}
	if err := os.WriteFile(filepath.Join(staging, "extension.toml"), manifest, 0o644); err != nil {
		t.Fatalf("os.WriteFile(staged extension.toml) error = %v", err)
	}
	hash, err := extensionpkg.ComputeDirectoryChecksum(staging)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", staging, err)
	}
	generationDir := filepath.Join(dist, "gen-"+hash)
	if err := os.Rename(staging, generationDir); err != nil {
		t.Fatalf("os.Rename(%q, %q) error = %v", staging, generationDir, err)
	}
	return hash
}

func performSecretTransportRequest(
	t *testing.T,
	engine *gin.Engine,
	method string,
	path string,
	body []byte,
) *httptest.ResponseRecorder {
	t.Helper()
	reader := io.Reader(http.NoBody)
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func waitForSecretExtensionLogs(
	t *testing.T,
	service *daemonExtensionService,
	name string,
	actor taskpkg.ActorContext,
) []contract.ExtensionLogPayload {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		logs, err := service.ExtensionLogs(t.Context(), name, 0, actor)
		if err != nil {
			t.Fatalf("ExtensionLogs() error = %v", err)
		}
		for _, entry := range logs {
			if strings.Contains(entry.Message, "safe=visible") {
				return logs
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("extension log ring did not receive the helper stderr record")
	return nil
}

func readSecretExtensionSSEFrame(t *testing.T, engine *gin.Engine, name string) string {
	t.Helper()
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(t.Context())
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		server.URL+"/extensions/"+name+"/logs?follow=1",
		http.NoBody,
	)
	if err != nil {
		cancel()
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		cancel()
		t.Fatalf("client.Do(SSE) error = %v", err)
	}
	defer func() {
		cancel()
		if err := response.Body.Close(); err != nil {
			t.Errorf("SSE response body Close() error = %v", err)
		}
	}()
	if response.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			t.Fatalf("io.ReadAll(SSE error body) error = %v", readErr)
		}
		t.Fatalf("SSE status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, body)
	}
	reader := bufio.NewReader(response.Body)
	var frame strings.Builder
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			t.Fatalf("ReadString(SSE) error = %v", readErr)
		}
		frame.WriteString(line)
		if line == "\n" {
			return frame.String()
		}
	}
}

func newSecretEchoingPublishFailureServer(t *testing.T, secrets []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		if _, err := fmt.Fprintf(writer, "upstream failure %s %s", secrets[0], secrets[1]); err != nil {
			t.Errorf("fmt.Fprintf(publish failure response) error = %v", err)
		}
	}))
}

func mustSecretTransportJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%T) error = %v", value, err)
	}
	return payload
}

func assertSecretsAbsent(t *testing.T, surface, value string, secrets []string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(value, secret) {
			t.Fatalf("%s leaked secret %q: %s", surface, secret, value)
		}
	}
}
