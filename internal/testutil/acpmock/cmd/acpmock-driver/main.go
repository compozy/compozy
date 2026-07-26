package main

import (
	"context"

	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/agh/internal/testutil/acpmock"
)

var (
	_ acpsdk.Agent       = (*mockAgent)(nil)
	_ acpsdk.AgentLoader = (*mockAgent)(nil)
)

type cliArgs struct {
	FixturePath     string
	AgentName       string
	DiagnosticsPath string
}

type sessionState struct {
	PromptCount          int
	ConfigOptions        []acpsdk.SessionConfigOption
	activePromptCancel   context.CancelFunc
	activePromptCancelID uint64
}

type mockAgent struct {
	conn            agentConnection
	agent           acpmock.AgentFixture
	configTemplate  []acpsdk.SessionConfigOption
	diagnosticsPath string
	lifecycleCtx    context.Context
	cancelLifecycle context.CancelFunc

	mu          sync.Mutex
	sessions    map[string]*sessionState
	nextSession int
	nextCancel  uint64
	asyncWG     sync.WaitGroup
}

type agentConnection interface {
	SessionUpdate(context.Context, acpsdk.SessionNotification) error
	RequestPermission(context.Context, acpsdk.RequestPermissionRequest) (acpsdk.RequestPermissionResponse, error)
	CreateTerminal(context.Context, acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error)
	WaitForTerminalExit(context.Context, acpsdk.WaitForTerminalExitRequest) (acpsdk.WaitForTerminalExitResponse, error)
	TerminalOutput(context.Context, acpsdk.TerminalOutputRequest) (acpsdk.TerminalOutputResponse, error)
	ReleaseTerminal(context.Context, acpsdk.ReleaseTerminalRequest) (acpsdk.ReleaseTerminalResponse, error)
}

type sandboxRunResult struct {
	Output        string
	ExitCode      *int
	ObservedError string
}

func main() {
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	fixture, err := acpmock.LoadFixture(args.FixturePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	agentFixture, err := fixture.Agent(args.AgentName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	lifecycleCtx, cancelLifecycle := context.WithCancel(context.Background())

	agent := &mockAgent{
		agent:           agentFixture,
		configTemplate:  sessionConfigOptionsFromFixture(agentFixture.ConfigOptions),
		diagnosticsPath: strings.TrimSpace(args.DiagnosticsPath),
		lifecycleCtx:    lifecycleCtx,
		cancelLifecycle: cancelLifecycle,
		sessions:        make(map[string]*sessionState),
	}
	conn := acpsdk.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.SetAgentConnection(conn)
	<-conn.Done()
	cancelLifecycle()
	agent.waitForAsyncControls()
}

func parseArgs(argv []string) (cliArgs, error) {
	fs := flag.NewFlagSet("acpmock-driver", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var args cliArgs
	fs.StringVar(&args.FixturePath, "fixture", "", "fixture JSON path")
	fs.StringVar(&args.AgentName, "agent", "", "fixture agent name")
	fs.StringVar(&args.DiagnosticsPath, "diagnostics", "", "diagnostics jsonl path")

	if err := fs.Parse(argv); err != nil {
		return cliArgs{}, err
	}
	if strings.TrimSpace(args.FixturePath) == "" {
		return cliArgs{}, errors.New("acpmock-driver: --fixture is required")
	}
	if strings.TrimSpace(args.AgentName) == "" {
		return cliArgs{}, errors.New("acpmock-driver: --agent is required")
	}
	return args, nil
}

func (a *mockAgent) SetAgentConnection(conn *acpsdk.AgentSideConnection) {
	a.conn = conn
}

func (a *mockAgent) Authenticate(context.Context, acpsdk.AuthenticateRequest) (acpsdk.AuthenticateResponse, error) {
	return acpsdk.AuthenticateResponse{}, nil
}

func (a *mockAgent) Initialize(context.Context, acpsdk.InitializeRequest) (acpsdk.InitializeResponse, error) {
	return acpsdk.InitializeResponse{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		AgentCapabilities: acpsdk.AgentCapabilities{
			LoadSession: a.agent.SupportsLoadSession(),
		},
		AuthMethods: []acpsdk.AuthMethod{},
	}, nil
}

func (a *mockAgent) Cancel(_ context.Context, params acpsdk.CancelNotification) error {
	sessionID := strings.TrimSpace(string(params.SessionId))
	if sessionID == "" {
		return errors.New("acpmock-driver: session id is required")
	}

	a.mu.Lock()
	session := a.sessions[sessionID]
	var cancel context.CancelFunc
	if session != nil {
		cancel = session.activePromptCancel
	}
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (a *mockAgent) CloseSession(
	context.Context,
	acpsdk.CloseSessionRequest,
) (acpsdk.CloseSessionResponse, error) {
	return acpsdk.CloseSessionResponse{}, nil
}

func (a *mockAgent) ListSessions(
	context.Context,
	acpsdk.ListSessionsRequest,
) (acpsdk.ListSessionsResponse, error) {
	return acpsdk.ListSessionsResponse{}, nil
}

func (a *mockAgent) ResumeSession(
	_ context.Context,
	params acpsdk.ResumeSessionRequest,
) (acpsdk.ResumeSessionResponse, error) {
	sessionID := strings.TrimSpace(string(params.SessionId))
	if sessionID == "" {
		return acpsdk.ResumeSessionResponse{}, errors.New("acpmock-driver: session id is required")
	}
	return acpsdk.ResumeSessionResponse{
		ConfigOptions: a.sessionConfigOptions(sessionID),
	}, nil
}

func (a *mockAgent) NewSession(_ context.Context, params acpsdk.NewSessionRequest) (acpsdk.NewSessionResponse, error) {
	a.mu.Lock()
	a.nextSession++
	sessionID := fmt.Sprintf("%s-session-%d", a.agent.Name, a.nextSession)
	a.sessions[sessionID] = &sessionState{
		ConfigOptions: cloneSessionConfigOptions(a.configTemplate),
	}
	a.mu.Unlock()
	if err := a.writeSessionDiagnostics("session_new", sessionID, params.McpServers); err != nil {
		return acpsdk.NewSessionResponse{}, err
	}
	return acpsdk.NewSessionResponse{
		SessionId:     acpsdk.SessionId(sessionID),
		ConfigOptions: a.sessionConfigOptions(sessionID),
	}, nil
}

func (a *mockAgent) LoadSession(
	_ context.Context,
	params acpsdk.LoadSessionRequest,
) (acpsdk.LoadSessionResponse, error) {
	a.mu.Lock()
	sessionID := strings.TrimSpace(string(params.SessionId))
	if sessionID == "" {
		a.mu.Unlock()
		return acpsdk.LoadSessionResponse{}, errors.New("acpmock-driver: session id is required")
	}
	if a.sessions[sessionID] == nil {
		a.sessions[sessionID] = &sessionState{
			ConfigOptions: cloneSessionConfigOptions(a.configTemplate),
		}
	}
	configOptions := cloneSessionConfigOptions(a.sessions[sessionID].ConfigOptions)
	a.mu.Unlock()
	if err := a.writeSessionDiagnostics("session_load", sessionID, params.McpServers); err != nil {
		return acpsdk.LoadSessionResponse{}, err
	}
	return acpsdk.LoadSessionResponse{ConfigOptions: configOptions}, nil
}

func (a *mockAgent) SetSessionMode(
	context.Context,
	acpsdk.SetSessionModeRequest,
) (acpsdk.SetSessionModeResponse, error) {
	return acpsdk.SetSessionModeResponse{}, nil
}

func (a *mockAgent) Logout(context.Context, acpsdk.LogoutRequest) (acpsdk.LogoutResponse, error) {
	return acpsdk.LogoutResponse{}, nil
}

func (a *mockAgent) SetSessionConfigOption(
	_ context.Context,
	request acpsdk.SetSessionConfigOptionRequest,
) (acpsdk.SetSessionConfigOptionResponse, error) {
	if request.ValueId == nil {
		return acpsdk.SetSessionConfigOptionResponse{}, errors.New(
			"acpmock-driver: only value-id session config options are supported",
		)
	}
	sessionID := string(request.ValueId.SessionId)
	configID := string(request.ValueId.ConfigId)
	value := string(request.ValueId.Value)
	if err := a.setConfigOptionValue(sessionID, configID, value); err != nil {
		return acpsdk.SetSessionConfigOptionResponse{}, err
	}
	if err := a.writeProtocolDiagnostics(
		acpsdk.AgentMethodSessionSetConfigOption,
		sessionID,
		configID,
		value,
	); err != nil {
		return acpsdk.SetSessionConfigOptionResponse{}, err
	}
	return acpsdk.SetSessionConfigOptionResponse{
		ConfigOptions: a.sessionConfigOptions(sessionID),
	}, nil
}

func sessionConfigOptionsFromFixture(
	options []acpmock.SessionConfigOptionFixture,
) []acpsdk.SessionConfigOption {
	if len(options) == 0 {
		return nil
	}
	result := make([]acpsdk.SessionConfigOption, 0, len(options))
	for _, option := range options {
		values := make(acpsdk.SessionConfigSelectOptionsUngrouped, 0, len(option.Values))
		for _, value := range option.Values {
			label := strings.TrimSpace(value.Label)
			if label == "" {
				label = strings.TrimSpace(value.Value)
			}
			values = append(values, acpsdk.SessionConfigSelectOption{
				Name:  label,
				Value: acpsdk.SessionConfigValueId(strings.TrimSpace(value.Value)),
			})
		}
		result = append(result, acpsdk.SessionConfigOption{
			Select: &acpsdk.SessionConfigOptionSelect{
				Id:           acpsdk.SessionConfigId(strings.TrimSpace(option.ID)),
				Name:         strings.TrimSpace(option.Name),
				CurrentValue: acpsdk.SessionConfigValueId(strings.TrimSpace(option.Current)),
				Options: acpsdk.SessionConfigSelectOptions{
					Ungrouped: &values,
				},
				Type: "select",
			},
		})
	}
	return result
}

func (a *mockAgent) setConfigOptionValue(sessionID string, configID string, value string) error {
	trimmedSessionID := strings.TrimSpace(sessionID)
	trimmedConfigID := strings.TrimSpace(configID)
	trimmedValue := strings.TrimSpace(value)
	if trimmedSessionID == "" {
		return errors.New("acpmock-driver: session id is required")
	}
	if trimmedConfigID == "" {
		return errors.New("acpmock-driver: session config option id is required")
	}
	if trimmedValue == "" {
		return errors.New("acpmock-driver: session config option value is required")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	session := a.sessions[trimmedSessionID]
	if session == nil {
		session = &sessionState{
			ConfigOptions: cloneSessionConfigOptions(a.configTemplate),
		}
		a.sessions[trimmedSessionID] = session
	}
	for idx := range session.ConfigOptions {
		option := session.ConfigOptions[idx].Select
		if option == nil || string(option.Id) != trimmedConfigID {
			continue
		}
		if option.Options.Ungrouped == nil {
			return fmt.Errorf("acpmock-driver: config option %q has no selectable values", trimmedConfigID)
		}
		for _, candidate := range *option.Options.Ungrouped {
			if string(candidate.Value) == trimmedValue {
				option.CurrentValue = acpsdk.SessionConfigValueId(trimmedValue)
				return nil
			}
		}
		return fmt.Errorf(
			"acpmock-driver: config option %q value %q is not available",
			trimmedConfigID,
			trimmedValue,
		)
	}
	return fmt.Errorf("acpmock-driver: config option %q is not available", trimmedConfigID)
}

func (a *mockAgent) ensureSessionState(sessionID string) *sessionState {
	trimmedSessionID := strings.TrimSpace(sessionID)
	a.mu.Lock()
	defer a.mu.Unlock()
	session := a.sessions[trimmedSessionID]
	if session == nil {
		session = &sessionState{
			ConfigOptions: cloneSessionConfigOptions(a.configTemplate),
		}
		a.sessions[trimmedSessionID] = session
	}
	return session
}

func (a *mockAgent) sessionConfigOptions(sessionID string) []acpsdk.SessionConfigOption {
	session := a.ensureSessionState(sessionID)
	a.mu.Lock()
	defer a.mu.Unlock()
	return cloneSessionConfigOptions(session.ConfigOptions)
}

func cloneSessionConfigOptions(options []acpsdk.SessionConfigOption) []acpsdk.SessionConfigOption {
	if len(options) == 0 {
		return nil
	}
	cloned := make([]acpsdk.SessionConfigOption, 0, len(options))
	for _, option := range options {
		if option.Select != nil {
			selectCopy := *option.Select
			if option.Select.Options.Ungrouped != nil {
				values := append(acpsdk.SessionConfigSelectOptionsUngrouped(nil), (*option.Select.Options.Ungrouped)...)
				selectCopy.Options.Ungrouped = &values
			}
			cloned = append(cloned, acpsdk.SessionConfigOption{Select: &selectCopy})
			continue
		}
		if option.Boolean != nil {
			booleanCopy := *option.Boolean
			cloned = append(cloned, acpsdk.SessionConfigOption{Boolean: &booleanCopy})
		}
	}
	return cloned
}
