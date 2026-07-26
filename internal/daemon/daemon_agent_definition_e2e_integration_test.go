//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestDaemonE2EAgentDefinitionLifecycleParity(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should preserve lifecycle parity across CLI HTTP and restart", func(t *testing.T) {
		// not parallel: lifecycle steps share one ordered runtime state.
		runDaemonE2EAgentDefinitionLifecycleParity(t)
	})

	t.Run("Should categorize and update a bundled extension agent through the effective catalog", func(t *testing.T) {
		// not parallel: lifecycle steps share one ordered runtime state.
		configSeed := agentDefinitionE2EConfigSeed(t)
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			ConfigSeed: configSeed,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		const agentName = "code_implementer"
		var before aghcontract.AgentResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodGet,
			"/api/agents/"+url.PathEscape(agentName),
			nil,
			&before,
		); err != nil {
			t.Fatalf("HTTP get extension agent error = %v", err)
		}
		if !reflect.DeepEqual(before.Agent.CategoryPath, []string{"Compozy"}) {
			t.Fatalf("extension agent category = %#v, want Compozy", before.Agent.CategoryPath)
		}
		if before.Agent.EffectiveRuntime == nil || before.Agent.EffectiveRuntime.Provider == "" {
			t.Fatalf("extension agent effective runtime = %#v, want resolved provider", before.Agent.EffectiveRuntime)
		}
		selectedProvider := before.Agent.EffectiveRuntime.Provider
		selectedModel := before.Agent.EffectiveRuntime.Model

		update := aghcontract.UpdateAgentRequest{
			Agent: aghcontract.CreateAgentPayload{
				Name:            before.Agent.Name,
				Provider:        selectedProvider,
				Command:         before.Agent.Command,
				Model:           selectedModel,
				ReasoningEffort: before.Agent.ReasoningEffort,
				Tools:           append([]string(nil), before.Agent.Tools...),
				Toolsets:        append([]string(nil), before.Agent.Toolsets...),
				DenyTools:       append([]string(nil), before.Agent.DenyTools...),
				Permissions:     aghcontract.SettingsPermissionMode(before.Agent.Permissions),
				CategoryPath:    append([]string(nil), before.Agent.CategoryPath...),
				Skills:          before.Agent.Skills,
				Prompt:          before.Agent.Prompt,
			},
			ExpectedDigest: before.Agent.DefinitionDigest,
		}
		var updated aghcontract.AgentResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodPut,
			"/api/agents/"+url.PathEscape(agentName),
			update,
			&updated,
		); err != nil {
			t.Fatalf("HTTP update extension agent error = %v", err)
		}
		if updated.Agent.Provider != selectedProvider || updated.Agent.Model != selectedModel ||
			!reflect.DeepEqual(updated.Agent.CategoryPath, []string{"Compozy"}) {
			t.Fatalf("updated extension agent = %#v, want selected runtime in Compozy", updated.Agent)
		}
		if updated.Agent.DefinitionDigest == before.Agent.DefinitionDigest {
			t.Fatalf("updated extension agent digest = %q, want a persisted runtime change", updated.Agent.DefinitionDigest)
		}

		var fresh aghcontract.AgentResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodGet,
			"/api/agents/"+url.PathEscape(agentName),
			nil,
			&fresh,
		); err != nil {
			t.Fatalf("HTTP fresh-read extension agent error = %v", err)
		}
		if fresh.Agent.DefinitionDigest != updated.Agent.DefinitionDigest ||
			fresh.Agent.Provider != selectedProvider || fresh.Agent.Model != selectedModel {
			t.Fatalf("fresh extension agent = %#v, want persisted update %#v", fresh.Agent, updated.Agent)
		}

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := harness.Stop(stopCtx); err != nil {
			stopCancel()
			t.Fatalf("stop runtime before extension agent restart check error = %v", err)
		}
		stopCancel()

		restarted := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			BinaryPath: harness.BinaryPath,
			HomePaths:  harness.HomePaths,
			ConfigSeed: configSeed,
			Workspace:  e2etest.WorkspaceSeedOptions{Root: harness.WorkspaceRoot},
		})
		var afterRestart aghcontract.AgentResponse
		if err := restarted.HTTPJSON(
			ctx,
			http.MethodGet,
			"/api/agents/"+url.PathEscape(agentName),
			nil,
			&afterRestart,
		); err != nil {
			t.Fatalf("HTTP post-restart extension agent read error = %v", err)
		}
		if afterRestart.Agent.DefinitionDigest != updated.Agent.DefinitionDigest ||
			afterRestart.Agent.Provider != selectedProvider || afterRestart.Agent.Model != selectedModel ||
			!reflect.DeepEqual(afterRestart.Agent.CategoryPath, []string{"Compozy"}) {
			t.Fatalf("post-restart extension agent = %#v, want persisted update %#v", afterRestart.Agent, updated.Agent)
		}
	})
}

func TestDaemonE2EReservedAgentNameSweep(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should reject every reserved authoring entry without changing the agent catalog", func(t *testing.T) {
		t.Parallel()
		runDaemonE2EReservedAgentNameSweep(t)
	})
}

func runDaemonE2EReservedAgentNameSweep(t *testing.T) {
	t.Helper()

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{
			AgentDefs: []e2etest.AgentSeed{{
				Name: "reserved-sweep-source", Provider: acpmock.ProviderName,
				Prompt: "Source for duplicate rejection.",
			}},
		},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "multi_agent_fixture.json"),
			FixtureAgent: "alpha",
			AgentName:    "reserved-sweep-provider",
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var before []aghcontract.AgentPayload
	if err := harness.CLI.RunJSONInDir(
		ctx,
		harness.WorkspaceRoot,
		&before,
		"agent",
		"list",
		"--workspace",
		harness.WorkspaceRoot,
		"-o",
		"json",
	); err != nil {
		t.Fatalf("CLI agent list before reserved sweep error = %v", err)
	}

	_, cliStderr, cliErr := harness.CLI.RunInDir(
		ctx,
		harness.WorkspaceRoot,
		"agent",
		"create",
		"coordinator",
		"--provider",
		acpmock.ProviderName,
		"--prompt",
		"Reserved CLI create.",
		"-o",
		"json",
	)
	assertReservedAgentCLIError(t, cliErr, cliStderr)

	httpCreate := agentDefinitionE2ERequestError(
		t,
		ctx,
		harness.HTTPClient,
		harness.HTTPURL("/api/agents"),
		http.MethodPost,
		aghcontract.CreateAgentRequest{
			Scope: aghcontract.AgentCreateScopeGlobal,
			Agent: aghcontract.CreateAgentPayload{
				Name: "dreaming-curator", Provider: acpmock.ProviderName, Prompt: "Reserved HTTP create.",
			},
		},
	)
	assertReservedAgentHTTPError(t, httpCreate)

	duplicate := agentDefinitionE2ERequestError(
		t,
		ctx,
		harness.UDSClient,
		harness.UDSURL("/api/agents/reserved-sweep-source/duplicate"),
		http.MethodPost,
		aghcontract.DuplicateAgentRequest{Name: "coordinator"},
	)
	assertReservedAgentHTTPError(t, duplicate)
	input, err := json.Marshal(map[string]string{
		"scope":     "workspace",
		"workspace": harness.WorkspaceRoot,
		"name":      "coordinator",
		"prompt":    "Reserved native create.",
	})
	if err != nil {
		t.Fatalf("json.Marshal(native reserved create) error = %v", err)
	}
	native := agentDefinitionE2EToolRequestError(
		t,
		ctx,
		harness.HTTPClient,
		harness.HTTPURL("/api/tools/agh__agent_create/invoke"),
		aghcontract.ToolInvokeRequest{
			WorkspaceID: harness.WorkspaceID,
			Input:       input,
		},
	)
	if native.Status != http.StatusUnprocessableEntity ||
		native.Payload.Error.Code != toolspkg.ErrorCodeAgentNameReserved {
		t.Fatalf("native reserved create = %#v, want 422 agent_name_reserved", native)
	}

	for _, name := range aghconfig.BuiltinAgentNames() {
		path := filepath.Join(harness.HomePaths.AgentsDir, name)
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(%q) error = %v, want os.ErrNotExist", path, err)
		}
	}
	workspaceReservedPath := filepath.Join(
		harness.WorkspaceRoot,
		aghconfig.DirName,
		aghconfig.AgentsDirName,
		"coordinator",
	)
	if _, err := os.Stat(workspaceReservedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(%q) error = %v, want os.ErrNotExist", workspaceReservedPath, err)
	}
	var after []aghcontract.AgentPayload
	if err := harness.CLI.RunJSONInDir(
		ctx,
		harness.WorkspaceRoot,
		&after,
		"agent",
		"list",
		"--workspace",
		harness.WorkspaceRoot,
		"-o",
		"json",
	); err != nil {
		t.Fatalf("CLI agent list after reserved sweep error = %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("agent catalog after reserved sweep = %#v, want unchanged %#v", after, before)
	}
}

func runDaemonE2EAgentDefinitionLifecycleParity(t *testing.T) {
	t.Helper()

	configSeed := agentDefinitionE2EConfigSeed(t)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: configSeed,
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "multi_agent_fixture.json"),
			FixtureAgent: "alpha",
			AgentName:    "parity-provider-fixture",
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	const sourceName = "parity-coder"
	var cliCreated aghcontract.AgentPayload
	if err := harness.CLI.RunJSONInDir(
		ctx,
		harness.WorkspaceRoot,
		&cliCreated,
		"agent",
		"create",
		sourceName,
		"--workspace",
		harness.WorkspaceRoot,
		"--provider",
		acpmock.ProviderName,
		"--model",
		"model-v1",
		"--tool",
		"builtin__shell",
		"--prompt",
		"Write code.",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("CLI agent create error = %v", err)
	}

	t.Run("Should expose a CLI create through HTTP", func(t *testing.T) {
		// not parallel: lifecycle steps share one ordered runtime state.
		var response aghcontract.AgentResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodGet,
			agentDefinitionE2EPath(sourceName, harness.WorkspaceRoot),
			nil,
			&response,
		); err != nil {
			t.Fatalf("HTTP get CLI-created agent error = %v", err)
		}
		if !reflect.DeepEqual(agentDefinitionE2EProjection(response.Agent), agentDefinitionE2EProjection(cliCreated)) {
			t.Fatalf("HTTP agent = %#v, want CLI parity %#v", response.Agent, cliCreated)
		}
	})

	updateRequest := aghcontract.UpdateAgentRequest{
		Workspace: harness.WorkspaceRoot,
		Agent: aghcontract.CreateAgentPayload{
			Name: sourceName, Provider: acpmock.ProviderName, Model: "model-v2",
			Tools: []string{"builtin__shell"}, Prompt: "Review code.",
		},
		ExpectedDigest: cliCreated.DefinitionDigest,
	}
	var httpUpdated aghcontract.AgentResponse
	if err := harness.HTTPJSON(
		ctx,
		http.MethodPut,
		"/api/agents/"+url.PathEscape(sourceName),
		updateRequest,
		&httpUpdated,
	); err != nil {
		t.Fatalf("HTTP agent update error = %v", err)
	}

	t.Run("Should expose an HTTP update through CLI info", func(t *testing.T) {
		// not parallel: lifecycle steps share one ordered runtime state.
		var cliInfo aghcontract.AgentPayload
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&cliInfo,
			"agent",
			"info",
			sourceName,
			"--workspace",
			harness.WorkspaceRoot,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI agent info error = %v", err)
		}
		if !reflect.DeepEqual(agentDefinitionE2EProjection(cliInfo), agentDefinitionE2EProjection(httpUpdated.Agent)) {
			t.Fatalf("CLI info = %#v, want HTTP parity %#v", cliInfo, httpUpdated.Agent)
		}
	})

	const duplicateName = "parity-reviewer"
	duplicateRequest := aghcontract.DuplicateAgentRequest{
		Name:      duplicateName,
		Workspace: harness.WorkspaceRoot,
		Overrides: &aghcontract.DuplicateAgentOverrides{Model: "model-review"},
	}
	var udsDuplicated aghcontract.AgentResponse
	if err := harness.UDSJSON(
		ctx,
		http.MethodPost,
		"/api/agents/"+url.PathEscape(sourceName)+"/duplicate",
		duplicateRequest,
		&udsDuplicated,
	); err != nil {
		t.Fatalf("UDS agent duplicate error = %v", err)
	}

	t.Run("Should expose an identical UDS duplicate through every read surface", func(t *testing.T) {
		// not parallel: lifecycle steps share one ordered runtime state.
		var httpInfo aghcontract.AgentResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodGet,
			agentDefinitionE2EPath(duplicateName, harness.WorkspaceRoot),
			nil,
			&httpInfo,
		); err != nil {
			t.Fatalf("HTTP duplicated agent info error = %v", err)
		}
		var udsInfo aghcontract.AgentResponse
		if err := harness.UDSJSON(
			ctx,
			http.MethodGet,
			agentDefinitionE2EPath(duplicateName, harness.WorkspaceRoot),
			nil,
			&udsInfo,
		); err != nil {
			t.Fatalf("UDS duplicated agent info error = %v", err)
		}
		var cliInfo aghcontract.AgentPayload
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&cliInfo,
			"agent",
			"info",
			duplicateName,
			"--workspace",
			harness.WorkspaceRoot,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI duplicated agent info error = %v", err)
		}
		want := agentDefinitionE2EProjection(udsDuplicated.Agent)
		for surface, got := range map[string]agentDefinitionE2EView{
			"HTTP": agentDefinitionE2EProjection(httpInfo.Agent),
			"UDS":  agentDefinitionE2EProjection(udsInfo.Agent),
			"CLI":  agentDefinitionE2EProjection(cliInfo),
		} {
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s duplicate = %#v, want %#v", surface, got, want)
			}
		}
	})

	conflictPath := "/api/agents/" + url.PathEscape(sourceName) + "/duplicate"
	httpConflict := agentDefinitionE2ERequestError(
		t,
		ctx,
		harness.HTTPClient,
		harness.HTTPURL(conflictPath),
		http.MethodPost,
		duplicateRequest,
	)
	udsConflict := agentDefinitionE2ERequestError(
		t,
		ctx,
		harness.UDSClient,
		harness.UDSURL(conflictPath),
		http.MethodPost,
		duplicateRequest,
	)
	if httpConflict.Status != http.StatusConflict || udsConflict.Status != http.StatusConflict ||
		httpConflict.Payload.Error != udsConflict.Payload.Error {
		t.Fatalf("duplicate conflicts = HTTP %#v / UDS %#v, want identical 409", httpConflict, udsConflict)
	}
	_, cliConflictStderr, cliConflictErr := harness.CLI.RunInDir(
		ctx,
		harness.WorkspaceRoot,
		"agent",
		"duplicate",
		sourceName,
		duplicateName,
		"--workspace",
		harness.WorkspaceRoot,
		"-o",
		"json",
	)
	assertAgentDefinitionE2ECLIError(
		t,
		cliConflictErr,
		cliConflictStderr,
		agentidentity.ExitIdentityInvalid,
		httpConflict.Payload.Error,
	)

	var deleted aghcontract.DeleteAgentResponse
	if err := harness.CLI.RunJSONInDir(
		ctx,
		harness.WorkspaceRoot,
		&deleted,
		"agent",
		"delete",
		duplicateName,
		"--workspace",
		harness.WorkspaceRoot,
		"--yes",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("CLI agent delete error = %v", err)
	}
	if deleted.Name != duplicateName || deleted.Origin != aghcontract.AgentOriginWorkspace {
		t.Fatalf("CLI delete = %#v, want workspace duplicate", deleted)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := harness.Stop(stopCtx); err != nil {
		stopCancel()
		t.Fatalf("stop runtime before restart error = %v", err)
	}
	stopCancel()

	restarted := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath: harness.BinaryPath,
		HomePaths:  harness.HomePaths,
		ConfigSeed: configSeed,
		Workspace:  e2etest.WorkspaceSeedOptions{Root: harness.WorkspaceRoot},
	})

	notFoundPath := agentDefinitionE2EPath(duplicateName, restarted.WorkspaceRoot)
	httpNotFound := agentDefinitionE2ERequestError(
		t,
		ctx,
		restarted.HTTPClient,
		restarted.HTTPURL(notFoundPath),
		http.MethodGet,
		nil,
	)
	udsNotFound := agentDefinitionE2ERequestError(
		t,
		ctx,
		restarted.UDSClient,
		restarted.UDSURL(notFoundPath),
		http.MethodGet,
		nil,
	)
	if httpNotFound.Status != http.StatusNotFound || udsNotFound.Status != http.StatusNotFound ||
		httpNotFound.Payload.Error != udsNotFound.Payload.Error {
		t.Fatalf("post-restart not found = HTTP %#v / UDS %#v, want identical 404", httpNotFound, udsNotFound)
	}
	_, cliNotFoundStderr, cliNotFoundErr := restarted.CLI.RunInDir(
		ctx,
		restarted.WorkspaceRoot,
		"agent",
		"info",
		duplicateName,
		"--workspace",
		restarted.WorkspaceRoot,
		"-o",
		"json",
	)
	assertAgentDefinitionE2ECLIError(
		t,
		cliNotFoundErr,
		cliNotFoundStderr,
		agentidentity.ExitUnavailable,
		httpNotFound.Payload.Error,
	)
}

func agentDefinitionE2EConfigSeed(t *testing.T) e2etest.ConfigSeedOptions {
	t.Helper()

	driverPath, err := acpmock.DefaultDriverPath()
	if err != nil {
		t.Fatalf("resolve ACP mock driver path: %v", err)
	}

	return e2etest.ConfigSeedOptions{
		DefaultProvider: acpmock.ProviderName,
		Providers: map[string]aghconfig.ProviderConfig{
			acpmock.ProviderName: acpmock.ProviderConfig(driverPath),
		},
	}
}

type agentDefinitionE2EView struct {
	Name             string
	Provider         string
	Model            string
	Prompt           string
	Origin           aghcontract.AgentOrigin
	WorkspaceID      string
	DefinitionDigest string
	Tools            []string
}

func agentDefinitionE2EProjection(agent aghcontract.AgentPayload) agentDefinitionE2EView {
	return agentDefinitionE2EView{
		Name:             agent.Name,
		Provider:         agent.Provider,
		Model:            agent.Model,
		Prompt:           agent.Prompt,
		Origin:           agent.Origin,
		WorkspaceID:      agent.WorkspaceID,
		DefinitionDigest: agent.DefinitionDigest,
		Tools:            append([]string(nil), agent.Tools...),
	}
}
func agentDefinitionE2EPath(name string, workspace string) string {
	return "/api/agents/" + url.PathEscape(name) + "?workspace=" + url.QueryEscape(workspace)
}

type agentDefinitionE2EError struct {
	Status  int
	Payload aghcontract.ErrorPayload
}

type agentDefinitionE2EToolError struct {
	Status  int
	Payload aghcontract.ToolErrorResponse
}

func agentDefinitionE2ERequestError(
	t *testing.T,
	ctx context.Context,
	client *http.Client,
	target string,
	method string,
	body any,
) agentDefinitionE2EError {
	t.Helper()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal(%s %s body) error = %v", method, target, err)
		}
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(%s %s) error = %v", method, target, err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("HTTP client %s %s error = %v", method, target, err)
	}
	payload, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("read/close %s %s response error = %v", method, target, err)
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		t.Fatalf("%s %s status = %d, want error; body=%s", method, target, response.StatusCode, payload)
	}
	var errorPayload aghcontract.ErrorPayload
	if err := json.Unmarshal(payload, &errorPayload); err != nil {
		t.Fatalf("json.Unmarshal(%s %s error) = %v; body=%s", method, target, err, payload)
	}
	return agentDefinitionE2EError{Status: response.StatusCode, Payload: errorPayload}
}

func agentDefinitionE2EToolRequestError(
	t *testing.T,
	ctx context.Context,
	client *http.Client,
	target string,
	body aghcontract.ToolInvokeRequest,
) agentDefinitionE2EToolError {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal(native tool request) error = %v", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(native tool request) error = %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("HTTP native tool request error = %v", err)
	}
	responsePayload, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("read/close native tool response error = %v", err)
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		t.Fatalf("native tool status = %d, want error; body=%s", response.StatusCode, responsePayload)
	}
	var errorPayload aghcontract.ToolErrorResponse
	if err := json.Unmarshal(responsePayload, &errorPayload); err != nil {
		t.Fatalf("json.Unmarshal(native tool error) = %v; body=%s", err, responsePayload)
	}
	return agentDefinitionE2EToolError{Status: response.StatusCode, Payload: errorPayload}
}

func assertReservedAgentHTTPError(t *testing.T, got agentDefinitionE2EError) {
	t.Helper()

	if got.Status != http.StatusUnprocessableEntity || got.Payload.Diagnostic == nil ||
		got.Payload.Diagnostic.Code != aghcontract.CodeAgentNameReserved {
		t.Fatalf("reserved agent mutation = %#v, want 422 agent_name_reserved", got)
	}
}

func assertReservedAgentCLIError(t *testing.T, err error, stderr string) {
	t.Helper()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("reserved CLI error = %T %[1]v, want *exec.ExitError", err)
	}
	var payload aghcontract.ErrorPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &payload); err != nil {
		t.Fatalf("json.Unmarshal(reserved CLI stderr) = %v; stderr=%s", err, stderr)
	}
	if payload.Diagnostic == nil || payload.Diagnostic.Code != aghcontract.CodeAgentNameReserved {
		t.Fatalf("reserved CLI payload = %#v, want agent_name_reserved", payload)
	}
}

func assertAgentDefinitionE2ECLIError(
	t *testing.T,
	err error,
	stderr string,
	wantExit int,
	wantMessage string,
) {
	t.Helper()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("CLI error = %T %[1]v, want *exec.ExitError", err)
	}
	if exitErr.ExitCode() != wantExit {
		t.Fatalf("CLI exit code = %d, want %d; stderr=%s", exitErr.ExitCode(), wantExit, stderr)
	}
	var payload aghcontract.ErrorPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &payload); err != nil {
		t.Fatalf("json.Unmarshal(CLI stderr) = %v; stderr=%s", err, stderr)
	}
	if payload.Error != wantMessage {
		t.Fatalf("CLI error = %q, want %q", payload.Error, wantMessage)
	}
}
