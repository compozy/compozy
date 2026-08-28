package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kballard/go-shellquote"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/toolruntime"
)

const (
	testHelperEnvKey      = "COMPOZY_TEST_ACP_HELPER"
	testHelperScenarioKey = "COMPOZY_TEST_ACP_SCENARIO"
	testHelperFileKey     = "COMPOZY_TEST_ACP_FILE"
	testHelperCaptureKey  = "COMPOZY_TEST_ACP_CAPTURE_FILE"
	testWrapperEnvKey     = "COMPOZY_TEST_ACP_WRAPPER"
)

func readPromptActivityReport(t *testing.T, reports <-chan PromptActivityReport) PromptActivityReport {
	t.Helper()

	select {
	case report := <-reports:
		return report
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for prompt activity report")
	}
	return PromptActivityReport{}
}

func startHelperProcess(
	t *testing.T,
	driver *Driver,
	scenario string,
	filePath string,
	overrides StartOpts,
) *AgentProcess {
	t.Helper()

	command := helperCommand(t)
	opts := StartOpts{
		AgentName:   "helper",
		Command:     command,
		Cwd:         t.TempDir(),
		Env:         helperEnv(scenario, filePath),
		Permissions: compozyconfig.PermissionModeApproveAll,
	}
	if overrides.AgentName != "" {
		opts.AgentName = overrides.AgentName
	}
	if overrides.Command != "" {
		opts.Command = overrides.Command
	}
	if overrides.Cwd != "" {
		opts.Cwd = overrides.Cwd
	}
	if overrides.AdditionalDirs != nil {
		opts.AdditionalDirs = append([]string(nil), overrides.AdditionalDirs...)
	}
	if overrides.Env != nil {
		opts.Env = overrides.Env
	}
	if overrides.Permissions != "" {
		opts.Permissions = overrides.Permissions
	}
	if overrides.MCPServers != nil {
		opts.MCPServers = overrides.MCPServers
	}
	if overrides.SystemPrompt != "" {
		opts.SystemPrompt = overrides.SystemPrompt
	}
	if overrides.SystemPromptDelivery != "" {
		opts.SystemPromptDelivery = overrides.SystemPromptDelivery
	}
	if overrides.PreferredModel != "" {
		opts.PreferredModel = overrides.PreferredModel
	}
	if overrides.ReasoningEffort != "" {
		opts.ReasoningEffort = overrides.ReasoningEffort
	}
	opts.Speed = overrides.Speed
	opts.ACPOptions = CloneSessionConfigOptionSelections(overrides.ACPOptions)
	opts.RuntimeStrategy = overrides.RuntimeStrategy
	opts.LaunchModelID = overrides.LaunchModelID
	opts.ResumeSessionID = overrides.ResumeSessionID
	opts.Launcher = overrides.Launcher
	opts.ToolHost = overrides.ToolHost
	opts.ToolGateway = overrides.ToolGateway
	opts.ProviderName = overrides.ProviderName
	opts.ProviderConfig = overrides.ProviderConfig
	opts.ProviderAuthEnv = overrides.ProviderAuthEnv
	opts.ActivateMCPServers = overrides.ActivateMCPServers

	proc, err := driver.Start(testutil.Context(t), opts)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	return proc
}

func stopProcess(t *testing.T, driver *Driver, proc *AgentProcess) {
	t.Helper()
	if proc == nil {
		return
	}
	if err := driver.Stop(testutil.Context(t), proc); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

type failingToolRuntimeStore struct {
	updateErr error
	upserts   int
}

func (s *failingToolRuntimeStore) UpsertProcessRecord(context.Context, toolruntime.ProcessRecord) error {
	s.upserts++
	if s.upserts > 1 {
		return s.updateErr
	}
	return nil
}

func (s *failingToolRuntimeStore) UpdateProcessRecordState(
	context.Context,
	toolruntime.ProcessStateUpdate,
) error {
	return s.updateErr
}

func (s *failingToolRuntimeStore) ListProcessRecords(
	context.Context,
	toolruntime.ProcessQuery,
) ([]toolruntime.ProcessRecord, error) {
	return nil, nil
}

func waitForProcess(t *testing.T, proc *AgentProcess) error {
	t.Helper()
	select {
	case <-proc.Done():
		return proc.Wait()
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for process exit")
		return nil
	}
}

func collectEvents(t *testing.T, eventsCh <-chan AgentEvent) []AgentEvent {
	t.Helper()

	events := make([]AgentEvent, 0, 8)
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case event, ok := <-eventsCh:
			if !ok {
				return events
			}
			events = append(events, event)
		case <-timeout.C:
			t.Fatalf("timeout waiting for prompt events; collected %#v", events)
		}
	}
}

func helperCommand(t *testing.T) string {
	t.Helper()

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	return shellquote.Join(bin, "-test.run=TestACPHelperProcess")
}

func helperEnv(scenario string, filePath string) []string {
	env := append([]string(nil), os.Environ()...)
	env = append(env,
		testHelperEnvKey+"=1",
		testHelperScenarioKey+"="+scenario,
	)
	if filePath != "" {
		env = append(env, testHelperFileKey+"="+filePath)
	}
	return env
}

func helperEnvWithCapture(scenario string, filePath string, capturePath string) []string {
	env := helperEnv(scenario, filePath)
	if strings.TrimSpace(capturePath) != "" {
		env = append(env, testHelperCaptureKey+"="+capturePath)
	}
	return env
}

type capturedRequestEnvelope struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type capturedNewSessionRequest struct {
	Cwd            string             `json:"cwd"`
	AdditionalDirs []string           `json:"additional_dirs,omitempty"`
	MCPServers     []acpsdk.McpServer `json:"mcpServers"`
}

type capturedLoadSessionRequest struct {
	Cwd            string             `json:"cwd"`
	AdditionalDirs []string           `json:"additional_dirs,omitempty"`
	MCPServers     []acpsdk.McpServer `json:"mcpServers"`
	SessionID      string             `json:"sessionId"`
}

type capturedSetSessionModeRequest struct {
	SessionID string `json:"sessionId"`
	ModeID    string `json:"modeId"`
}

type capturedSetSessionConfigOptionRequest struct {
	SessionID string
	ConfigID  string
	Type      string
	Value     string
	BoolValue *bool
}

func captureRequestParams(t *testing.T, path string, method string) map[string]json.RawMessage {
	t.Helper()

	matches := captureRequestParamsForMethod(t, path, method)
	if len(matches) > 0 {
		return matches[0]
	}
	t.Fatalf("capture file %q does not contain method %q", path, method)
	return nil
}

func captureMethodExists(t *testing.T, path string, method string) bool {
	t.Helper()

	return len(captureRequestParamsForMethod(t, path, method)) > 0
}

func captureRequestParamsForMethod(t *testing.T, path string, method string) []map[string]json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}

	matches := make([]map[string]json.RawMessage, 0)
	lines := strings.SplitSeq(strings.TrimSpace(string(data)), "\n")
	for line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var envelope capturedRequestEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(captured envelope) error = %v", err)
		}
		if envelope.Method != method {
			continue
		}

		var params map[string]json.RawMessage
		if err := json.Unmarshal(envelope.Params, &params); err != nil {
			t.Fatalf("json.Unmarshal(captured params) error = %v", err)
		}
		matches = append(matches, params)
	}
	return matches
}

func captureNegotiationSequence(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	sequence := make([]string, 0, 3)
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var envelope capturedRequestEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(captured envelope) error = %v", err)
		}
		var params map[string]json.RawMessage
		switch envelope.Method {
		case acpsdk.AgentMethodSessionSetMode:
			if err := json.Unmarshal(envelope.Params, &params); err != nil {
				t.Fatalf("json.Unmarshal(set-mode params) error = %v", err)
			}
			request := decodeCapturedSetSessionModeRequest(t, params)
			sequence = append(sequence, "mode:"+request.ModeID)
		case acpsdk.AgentMethodSessionSetConfigOption:
			if err := json.Unmarshal(envelope.Params, &params); err != nil {
				t.Fatalf("json.Unmarshal(set-config params) error = %v", err)
			}
			request := decodeCapturedSetSessionConfigOptionRequest(t, params)
			sequence = append(sequence, request.ConfigID+":"+request.displayValue())
		}
	}
	return sequence
}

func decodeCapturedNewSessionRequest(t *testing.T, params map[string]json.RawMessage) capturedNewSessionRequest {
	t.Helper()

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal(new-session params) error = %v", err)
	}
	var request capturedNewSessionRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("json.Unmarshal(new-session request) error = %v", err)
	}
	return request
}

func decodeCapturedLoadSessionRequest(t *testing.T, params map[string]json.RawMessage) capturedLoadSessionRequest {
	t.Helper()

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal(load-session params) error = %v", err)
	}
	var request capturedLoadSessionRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("json.Unmarshal(load-session request) error = %v", err)
	}
	return request
}

func decodeCapturedSetSessionModeRequest(
	t *testing.T,
	params map[string]json.RawMessage,
) capturedSetSessionModeRequest {
	t.Helper()

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal(set-session-mode params) error = %v", err)
	}
	var request capturedSetSessionModeRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("json.Unmarshal(set-session-mode request) error = %v", err)
	}
	return request
}

func decodeCapturedSetSessionConfigOptionRequest(
	t *testing.T,
	params map[string]json.RawMessage,
) capturedSetSessionConfigOptionRequest {
	t.Helper()

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal(set-session-config-option params) error = %v", err)
	}
	var wireRequest struct {
		SessionID string          `json:"sessionId"`
		ConfigID  string          `json:"configId"`
		Type      string          `json:"type"`
		Value     json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &wireRequest); err != nil {
		t.Fatalf("json.Unmarshal(set-session-config-option request) error = %v", err)
	}
	request := capturedSetSessionConfigOptionRequest{
		SessionID: wireRequest.SessionID,
		ConfigID:  wireRequest.ConfigID,
		Type:      wireRequest.Type,
	}
	if err := json.Unmarshal(wireRequest.Value, &request.Value); err == nil {
		return request
	}
	var boolValue bool
	if err := json.Unmarshal(wireRequest.Value, &boolValue); err != nil {
		t.Fatalf("json.Unmarshal(set-session-config-option value) error = %v", err)
	}
	request.BoolValue = new(boolValue)
	return request
}

func (r capturedSetSessionConfigOptionRequest) displayValue() string {
	if r.BoolValue != nil {
		return strconv.FormatBool(*r.BoolValue)
	}
	return r.Value
}

func assertConfigOption(
	t *testing.T,
	options []SessionConfigOption,
	id string,
	current string,
	wantValues ...string,
) {
	t.Helper()

	var found *SessionConfigOption
	for index := range options {
		if options[index].ID == id {
			found = &options[index]
			break
		}
	}
	if found == nil {
		t.Fatalf("config option %q not found in %#v", id, options)
	}
	if got := found.CurrentValueID; got != current {
		t.Fatalf("config option %q current = %q, want %q", id, got, current)
	}
	values := make([]string, 0, len(found.Values))
	for _, value := range found.Values {
		values = append(values, value.Value)
	}
	for _, want := range wantValues {
		if !slices.Contains(values, want) {
			t.Fatalf("config option %q values = %#v, want value %q", id, values, want)
		}
	}
}

func mustCanonicalDir(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%q) error = %v", path, err)
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		t.Fatalf("filepath.Abs(%q) error = %v", resolved, err)
	}
	return filepath.Clean(absolute)
}

func assertPermissionResult(t *testing.T, err error, wantOK bool) {
	t.Helper()
	if wantOK && err != nil {
		t.Fatalf("authorize() error = %v, want nil", err)
	}
	if !wantOK && !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("authorize() error = %v, want ErrPermissionDenied", err)
	}
}

type helperACPAgent struct {
	conn            *acpsdk.AgentSideConnection
	scenario        string
	filePath        string
	configOptionsMu sync.Mutex
	configOptions   []acpsdk.SessionConfigOption
}

func (a *helperACPAgent) Authenticate(
	context.Context,
	acpsdk.AuthenticateRequest,
) (acpsdk.AuthenticateResponse, error) {
	return acpsdk.AuthenticateResponse{}, nil
}

func (a *helperACPAgent) Initialize(context.Context, acpsdk.InitializeRequest) (acpsdk.InitializeResponse, error) {
	return acpsdk.InitializeResponse{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		AgentCapabilities: acpsdk.AgentCapabilities{
			LoadSession: a.scenario == "load_session" || a.scenario == "load_session_error" ||
				a.scenario == "load_mode_mapping" || a.scenario == "load_config_options",
			PromptCapabilities: acpsdk.PromptCapabilities{
				Image: a.scenario == "prompt_capabilities_image" ||
					a.scenario == "echo_prompt_blocks",
				Audio: a.scenario == "prompt_capabilities_audio",
				EmbeddedContext: a.scenario == "prompt_capabilities_embedded_context" ||
					a.scenario == "echo_prompt_blocks",
			},
		},
		AuthMethods: []acpsdk.AuthMethod{},
	}, nil
}

func (a *helperACPAgent) Cancel(context.Context, acpsdk.CancelNotification) error {
	return nil
}

func (a *helperACPAgent) CloseSession(
	context.Context,
	acpsdk.CloseSessionRequest,
) (acpsdk.CloseSessionResponse, error) {
	return acpsdk.CloseSessionResponse{}, nil
}

func (a *helperACPAgent) Logout(context.Context, acpsdk.LogoutRequest) (acpsdk.LogoutResponse, error) {
	return acpsdk.LogoutResponse{}, nil
}

func (a *helperACPAgent) ListSessions(
	context.Context,
	acpsdk.ListSessionsRequest,
) (acpsdk.ListSessionsResponse, error) {
	return acpsdk.ListSessionsResponse{}, nil
}

func (a *helperACPAgent) ResumeSession(
	context.Context,
	acpsdk.ResumeSessionRequest,
) (acpsdk.ResumeSessionResponse, error) {
	return acpsdk.ResumeSessionResponse{}, nil
}

func (a *helperACPAgent) NewSession(context.Context, acpsdk.NewSessionRequest) (acpsdk.NewSessionResponse, error) {
	if a.scenario == "mode_mapping" {
		return acpsdk.NewSessionResponse{
			SessionId: "sess-new",
			Modes:     helperModeStateWithCurrent("default", "default", "plan", "bypassPermissions"),
		}, nil
	}
	if a.scenario == "cursor_mode_mapping" || a.scenario == "cursor_mode_current_agent" {
		current := "plan"
		if a.scenario == "cursor_mode_current_agent" {
			current = "agent"
		}
		configOptions := []acpsdk.SessionConfigOption{
			helperSelectConfigOption("mode", "Mode", current, "agent", "plan", "ask"),
		}
		a.setHelperConfigOptions(configOptions)
		return acpsdk.NewSessionResponse{
			SessionId:     "sess-new",
			Modes:         helperModeStateWithCurrent(current, "agent", "plan", "ask"),
			ConfigOptions: configOptions,
		}, nil
	}
	if a.scenario == "config_options" ||
		a.scenario == "config_options_reject_speed" ||
		a.scenario == "config_options_no_model" ||
		a.scenario == "config_options_no_reasoning" ||
		a.scenario == "launch_bound_model" ||
		a.scenario == "model_specific_config_options" ||
		a.scenario == "runtime_config_options" ||
		a.scenario == "config_option_update" {
		configOptions := helperConfigOptions("new-model", "medium")
		if a.scenario == "launch_bound_model" {
			const currentModel = "shared-provider-default[]"
			configOptions = []acpsdk.SessionConfigOption{
				helperSelectConfigOption("model", "Model", currentModel, currentModel),
			}
		}
		if a.scenario == "model_specific_config_options" {
			configOptions = append(
				helperModelConfigOptions("new-model"),
				helperSelectConfigOption("effort", "Reasoning effort", "low", "low"),
			)
		}
		if a.scenario == "config_options_no_model" {
			configOptions = []acpsdk.SessionConfigOption{
				helperSelectConfigOption(
					"reasoning_effort",
					"Reasoning effort",
					"medium",
					"minimal",
					"medium",
					"xhigh",
				),
			}
		}
		if a.scenario == "config_options_no_reasoning" {
			configOptions = helperModelConfigOptions("new-model")
		}
		a.setHelperConfigOptions(configOptions)
		modes := helperModeState("new-mode")
		if a.scenario == "model_specific_config_options" {
			modes = helperModeStateWithCurrent("default", "default", "plan", "bypassPermissions")
		}
		return acpsdk.NewSessionResponse{
			SessionId:     "sess-new",
			Modes:         modes,
			ConfigOptions: configOptions,
		}, nil
	}
	return acpsdk.NewSessionResponse{
		SessionId: "sess-new",
		Modes:     helperModeState("new-mode"),
	}, nil
}

func (a *helperACPAgent) LoadSession(context.Context, acpsdk.LoadSessionRequest) (acpsdk.LoadSessionResponse, error) {
	if a.scenario == "load_session_error" {
		return acpsdk.LoadSessionResponse{}, errors.New("load failed")
	}
	if a.scenario == "load_mode_mapping" {
		return acpsdk.LoadSessionResponse{
			Modes: helperModeStateWithCurrent("default", "default", "plan", "bypassPermissions"),
		}, nil
	}
	if a.scenario == "load_config_options" {
		configOptions := helperConfigOptions("loaded-model", "high")
		a.setHelperConfigOptions(configOptions)
		return acpsdk.LoadSessionResponse{
			Modes:         helperModeState("loaded-mode"),
			ConfigOptions: configOptions,
		}, nil
	}
	return acpsdk.LoadSessionResponse{
		Modes: helperModeState("loaded-mode"),
	}, nil
}

func (a *helperACPAgent) Prompt(ctx context.Context, params acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	switch a.scenario {
	case "crash_on_prompt":
		os.Exit(23)
	case "prompt_request_error_with_reason":
		return acpsdk.PromptResponse{}, &acpsdk.RequestError{
			Code:    -32000,
			Message: "Authentication required",
			Data: map[string]any{
				"reason_codes": []string{"mcp_auth_required"},
			},
		}
	case "block_prompt_until_cancel":
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText("blocking"),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
		<-ctx.Done()
		return acpsdk.PromptResponse{}, ctx.Err()
	case "echo_prompt":
		text := ""
		if len(params.Prompt) > 0 && params.Prompt[0].Text != nil {
			text = params.Prompt[0].Text.Text
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(text),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "echo_prompt_meta":
		data, err := json.Marshal(params.Meta)
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(string(data)),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "echo_prompt_blocks":
		data, err := json.Marshal(params.Prompt)
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(string(data)),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "config_option_update":
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update: acpsdk.SessionUpdate{
				ConfigOptionUpdate: &acpsdk.SessionConfigOptionUpdate{
					ConfigOptions: helperConfigOptions("other-model", "xhigh"),
				},
			},
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "tool_update_burst":
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update: acpsdk.StartToolCall(
				"tool-burst",
				"Read file",
				acpsdk.WithStartKind(acpsdk.ToolKindRead),
				acpsdk.WithStartStatus(acpsdk.ToolCallStatusInProgress),
			),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
		for range 1_100 {
			if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
				SessionId: params.SessionId,
				Update: acpsdk.UpdateToolCall(
					"tool-burst",
					acpsdk.WithUpdateStatus(acpsdk.ToolCallStatusInProgress),
				),
			}); sendErr != nil {
				return acpsdk.PromptResponse{}, sendErr
			}
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update: acpsdk.UpdateToolCall(
				"tool-burst",
				acpsdk.WithUpdateStatus(acpsdk.ToolCallStatusCompleted),
			),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "diverse_update_burst":
		for index := range 1_100 {
			toolCallID := fmt.Sprintf("tool-diverse-%04d", index)
			if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
				SessionId: params.SessionId,
				Update: acpsdk.StartToolCall(
					acpsdk.ToolCallId(toolCallID),
					"Read file",
					acpsdk.WithStartKind(acpsdk.ToolKindRead),
					acpsdk.WithStartStatus(acpsdk.ToolCallStatusInProgress),
				),
			}); sendErr != nil {
				return acpsdk.PromptResponse{}, sendErr
			}
		}
	case "fs_read":
		response, err := a.conn.ReadTextFile(ctx, acpsdk.ReadTextFileRequest{
			SessionId: params.SessionId,
			Path:      a.filePath,
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(response.Content),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "fs_write_terminal":
		if _, err := a.conn.WriteTextFile(ctx, acpsdk.WriteTextFileRequest{
			SessionId: params.SessionId,
			Path:      a.filePath,
			Content:   "from-write",
		}); err != nil {
			return acpsdk.PromptResponse{}, err
		}
		readResponse, err := a.conn.ReadTextFile(ctx, acpsdk.ReadTextFileRequest{
			SessionId: params.SessionId,
			Path:      a.filePath,
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(readResponse.Content),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}

		cwd, err := os.Getwd()
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		createResp, err := a.conn.CreateTerminal(ctx, acpsdk.CreateTerminalRequest{
			SessionId: params.SessionId,
			Command:   "sh",
			Args:      []string{"-c", "printf terminal-ok"},
			Cwd:       new(cwd),
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if _, err := a.conn.WaitForTerminalExit(ctx, acpsdk.WaitForTerminalExitRequest{
			SessionId:  params.SessionId,
			TerminalId: createResp.TerminalId,
		}); err != nil {
			return acpsdk.PromptResponse{}, err
		}
		outputResp, err := a.conn.TerminalOutput(ctx, acpsdk.TerminalOutputRequest{
			SessionId:  params.SessionId,
			TerminalId: createResp.TerminalId,
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(outputResp.Output),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "permission":
		title := "permission request"
		locationPath := a.filePath
		if locationPath == "" {
			locationPath = filepath.Join(string(filepath.Separator), "workspace", "demo.txt")
		}
		outcome, err := a.conn.RequestPermission(ctx, acpsdk.RequestPermissionRequest{
			SessionId: params.SessionId,
			Options: []acpsdk.PermissionOption{
				{OptionId: "allow-once", Name: "allow once", Kind: acpsdk.PermissionOptionKindAllowOnce},
				{OptionId: "allow-always", Name: "allow always", Kind: acpsdk.PermissionOptionKindAllowAlways},
				{OptionId: "reject-once", Name: "reject once", Kind: acpsdk.PermissionOptionKindRejectOnce},
				{OptionId: "reject-always", Name: "reject always", Kind: acpsdk.PermissionOptionKindRejectAlways},
			},
			ToolCall: acpsdk.ToolCallUpdate{
				ToolCallId: "tool-1",
				Title:      &title,
				Locations: []acpsdk.ToolCallLocation{
					{Path: locationPath},
				},
			},
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		selected := "canceled"
		if outcome.Outcome.Selected != nil {
			selected = string(outcome.Outcome.Selected.OptionId)
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(selected),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "network_guardrails":
		targetPath := a.filePath
		if targetPath == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return acpsdk.PromptResponse{}, err
			}
			targetPath = filepath.Join(cwd, "network-blocked.txt")
		}

		writeResult := "write_unexpected"
		if _, err := a.conn.WriteTextFile(ctx, acpsdk.WriteTextFileRequest{
			SessionId: params.SessionId,
			Path:      targetPath,
			Content:   "blocked",
		}); err != nil {
			writeResult = "write_blocked"
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(writeResult),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}

		shellResult := "shell_unexpected"
		if _, err := a.conn.CreateTerminal(ctx, acpsdk.CreateTerminalRequest{
			SessionId: params.SessionId,
			Command:   "sh",
			Args:      []string{"-c", "printf nope"},
		}); err != nil {
			shellResult = "shell_blocked"
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(shellResult),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}

		cwd, err := os.Getwd()
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		createResp, err := a.conn.CreateTerminal(ctx, acpsdk.CreateTerminalRequest{
			SessionId: params.SessionId,
			Command:   "compozy",
			Args:      []string{"network", "status"},
			Cwd:       new(cwd),
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if _, err := a.conn.WaitForTerminalExit(ctx, acpsdk.WaitForTerminalExitRequest{
			SessionId:  params.SessionId,
			TerminalId: createResp.TerminalId,
		}); err != nil {
			return acpsdk.PromptResponse{}, err
		}
		outputResp, err := a.conn.TerminalOutput(ctx, acpsdk.TerminalOutputRequest{
			SessionId:  params.SessionId,
			TerminalId: createResp.TerminalId,
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(outputResp.Output),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	default:
		updates := []acpsdk.SessionUpdate{
			acpsdk.UpdateAgentMessageText("hello"),
			acpsdk.UpdateAgentThoughtText("thinking"),
			acpsdk.StartToolCall(
				"tool-1",
				"Read file",
				acpsdk.WithStartKind(acpsdk.ToolKindRead),
				acpsdk.WithStartStatus(acpsdk.ToolCallStatusInProgress),
			),
			acpsdk.UpdateToolCall(
				"tool-1",
				acpsdk.WithUpdateStatus(acpsdk.ToolCallStatusCompleted),
				acpsdk.WithUpdateTitle("Read file"),
			),
		}
		for _, update := range updates {
			if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
				SessionId: params.SessionId,
				Update:    update,
			}); err != nil {
				return acpsdk.PromptResponse{}, err
			}
		}
	}

	return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonEndTurn}, nil
}

func (a *helperACPAgent) SetSessionMode(
	context.Context,
	acpsdk.SetSessionModeRequest,
) (acpsdk.SetSessionModeResponse, error) {
	return acpsdk.SetSessionModeResponse{}, nil
}

func (a *helperACPAgent) SetSessionConfigOption(
	_ context.Context,
	request acpsdk.SetSessionConfigOptionRequest,
) (acpsdk.SetSessionConfigOptionResponse, error) {
	a.configOptionsMu.Lock()
	defer a.configOptionsMu.Unlock()
	if request.ValueId != nil {
		configID := string(request.ValueId.ConfigId)
		value := acpsdk.SessionConfigValueId(strings.TrimSpace(string(request.ValueId.Value)))
		if a.scenario == "config_options_reject_speed" && configID == "speed" {
			return acpsdk.SetSessionConfigOptionResponse{}, errors.New("provider rejected speed")
		}
		if (a.scenario == "model_specific_config_options" || a.scenario == "runtime_config_options") &&
			configID == "model" {
			configOptions := append(
				helperModelConfigOptions(string(value)),
				helperSelectConfigOption("effort", "Reasoning effort", "none", "none", "max"),
			)
			if a.scenario == "runtime_config_options" {
				configOptions = append(
					configOptions,
					helperSpeedConfigOption("normal"),
					helperSelectConfigOption("context", "Context", "standard", "standard", "large"),
					helperBooleanConfigOption("thinking", "Thinking", false),
				)
			}
			a.configOptions = configOptions
			return acpsdk.SetSessionConfigOptionResponse{
				ConfigOptions: append([]acpsdk.SessionConfigOption(nil), a.configOptions...),
			}, nil
		}
		for index := range a.configOptions {
			if a.configOptions[index].Select == nil || string(a.configOptions[index].Select.Id) != configID {
				continue
			}
			a.configOptions[index].Select.CurrentValue = value
		}
	}
	if request.Boolean != nil {
		configID := string(request.Boolean.ConfigId)
		for index := range a.configOptions {
			if a.configOptions[index].Boolean == nil || string(a.configOptions[index].Boolean.Id) != configID {
				continue
			}
			a.configOptions[index].Boolean.CurrentValue = request.Boolean.Value
		}
	}
	return acpsdk.SetSessionConfigOptionResponse{
		ConfigOptions: append([]acpsdk.SessionConfigOption(nil), a.configOptions...),
	}, nil
}

func (a *helperACPAgent) setHelperConfigOptions(options []acpsdk.SessionConfigOption) {
	a.configOptionsMu.Lock()
	defer a.configOptionsMu.Unlock()
	a.configOptions = append([]acpsdk.SessionConfigOption(nil), options...)
}

func helperConfigOptions(modelCurrent string, reasoningCurrent string) []acpsdk.SessionConfigOption {
	options := helperModelConfigOptions(modelCurrent)
	options = append(
		options,
		helperSelectConfigOption(
			"reasoning_effort",
			"Reasoning effort",
			reasoningCurrent,
			"minimal",
			"none",
			"low",
			"medium",
			"high",
			"xhigh",
			"max",
		),
		helperSpeedConfigOption("normal"),
	)
	return options
}

func helperSpeedConfigOption(current string) acpsdk.SessionConfigOption {
	option := helperSelectConfigOption("speed", "Speed", current, "normal", "fast")
	category := acpsdk.SessionConfigOptionCategory(speedConfigCategory)
	option.Select.Category = &category
	return option
}

func helperBooleanConfigOption(id string, name string, current bool) acpsdk.SessionConfigOption {
	return acpsdk.SessionConfigOption{
		Boolean: &acpsdk.SessionConfigOptionBoolean{
			Id:           acpsdk.SessionConfigId(id),
			Name:         name,
			CurrentValue: current,
			Type:         string(SessionConfigOptionKindBoolean),
		},
	}
}

func sessionConfigOptionFromSDKForTest(t *testing.T, option acpsdk.SessionConfigOption) SessionConfigOption {
	t.Helper()
	converted, ok := sessionConfigOptionFromSDK(option)
	if !ok {
		t.Fatalf("sessionConfigOptionFromSDK(%#v) did not convert", option)
	}
	return converted
}

func reasoningACPProviderConfig() *compozyconfig.ProviderConfig {
	return &compozyconfig.ProviderConfig{
		Models: compozyconfig.ProviderModelsConfig{
			Reasoning: compozyconfig.ProviderReasoningConfig{Apply: compozyconfig.ReasoningApplyACPOption},
		},
	}
}

func helperModelConfigOptions(current string) []acpsdk.SessionConfigOption {
	return []acpsdk.SessionConfigOption{
		helperSelectConfigOption("model", "Model", current, "new-model", "loaded-model", "other-model"),
	}
}

func helperSelectConfigOption(
	id string,
	name string,
	current string,
	values ...string,
) acpsdk.SessionConfigOption {
	selectOptions := make(acpsdk.SessionConfigSelectOptionsUngrouped, 0, len(values))
	for _, value := range values {
		selectOptions = append(selectOptions, acpsdk.SessionConfigSelectOption{
			Value: acpsdk.SessionConfigValueId(value),
			Name:  value,
		})
	}
	return acpsdk.SessionConfigOption{
		Select: &acpsdk.SessionConfigOptionSelect{
			Id:           acpsdk.SessionConfigId(id),
			Name:         name,
			CurrentValue: acpsdk.SessionConfigValueId(current),
			Options: acpsdk.SessionConfigSelectOptions{
				Ungrouped: &selectOptions,
			},
			Type: "select",
		},
	}
}

func helperModeState(id string) *acpsdk.SessionModeState {
	return helperModeStateWithCurrent(id, id)
}

func helperModeStateWithCurrent(current string, available ...string) *acpsdk.SessionModeState {
	modes := make([]acpsdk.SessionMode, 0, len(available))
	for _, id := range available {
		modes = append(modes, acpsdk.SessionMode{
			Id:   acpsdk.SessionModeId(id),
			Name: id,
		})
	}
	return &acpsdk.SessionModeState{
		CurrentModeId:  acpsdk.SessionModeId(current),
		AvailableModes: modes,
	}
}
