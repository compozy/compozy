package acp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/store"
)

// authRequiredCode is the ACP peer refusal that gates interactive login: the
// disposable session probe hit an unauthenticated account.
const authRequiredCode = -32000

// ErrACPAuthRequired reports that a disposable ACP session probe was refused
// with an authentication-required peer error.
var ErrACPAuthRequired = errors.New("acp: provider authentication required")

// errAuthInboundRefused rejects filesystem, terminal, and permission work sent
// to a disposable authentication process.
var errAuthInboundRefused = errors.New("acp: authentication process refuses inbound work")

// AuthMethodUnavailableReason identifies why a configured ACP method cannot be
// used for explicit login.
type AuthMethodUnavailableReason string

const (
	// AuthMethodUnavailableAbsent means initialization advertised no entry with
	// the configured method ID.
	AuthMethodUnavailableAbsent AuthMethodUnavailableReason = "absent"
	// AuthMethodUnavailableTerminal means the configured ID is advertised as a
	// terminal method, which disposable login never executes.
	AuthMethodUnavailableTerminal AuthMethodUnavailableReason = "terminal"
	// AuthMethodUnavailableUnknown means the configured ID is advertised with a
	// type disposable login does not speak. Unknown types fail closed instead of
	// becoming agent methods.
	AuthMethodUnavailableUnknown AuthMethodUnavailableReason = "unknown"
)

// AuthMethodUnavailableError reports that the exact configured ACP method is
// not available as an agent method. No authenticate request was sent.
type AuthMethodUnavailableError struct {
	MethodID string
	Reason   AuthMethodUnavailableReason
}

func (e *AuthMethodUnavailableError) Error() string {
	if e == nil {
		return "acp: ACP auth method unavailable"
	}
	return fmt.Sprintf(
		"acp: ACP auth method %q unavailable: %s",
		e.MethodID,
		e.Reason,
	)
}

// IsAuthRequiredError reports whether err carries an ACP authentication-required
// peer refusal, including through the driver's wrapped control-request chain.
func IsAuthRequiredError(err error) bool {
	reqErr, ok := errors.AsType[*acpsdk.RequestError](err)
	return ok && reqErr != nil && reqErr.Code == authRequiredCode
}

// AuthenticationRequest describes one disposable explicit-login attempt. The
// command, environment, and home arrive exactly as the caller resolved them;
// InteractiveOutput is the only sink for live login progress.
type AuthenticationRequest struct {
	SessionInspectionRequest
	MethodID          string
	InteractiveOutput io.Writer // local login only; never a persisted logger
}

// Authenticate performs one disposable explicit login: it probes readiness with
// a promptless session/new, authenticates the exact configured advertised
// agent method only when that probe reports auth-required, then verifies
// readiness once. A refused post-login session stays a classified failure.
func Authenticate(ctx context.Context, req AuthenticationRequest) error {
	if strings.TrimSpace(req.MethodID) == "" {
		return errors.New("acp: auth method is required")
	}
	if err := compozyconfig.ValidateACPMethod(req.MethodID, "acp: auth method"); err != nil {
		return err
	}
	return runAuthSession(ctx, authSessionParams{
		inspection:   req.SessionInspectionRequest,
		methodID:     strings.TrimSpace(req.MethodID),
		authenticate: true,
		output:       req.InteractiveOutput,
	})
}

// ProbeAuthentication verifies readiness with one promptless disposable
// session/new. It never sends authenticate, prompts, or tools; MCP servers
// stay an empty list so strict agents accept the probe.
func ProbeAuthentication(ctx context.Context, req SessionInspectionRequest) error {
	return runAuthSession(ctx, authSessionParams{
		inspection:   req,
		authenticate: false,
	})
}

type authSessionParams struct {
	inspection   SessionInspectionRequest
	methodID     string
	authenticate bool
	output       io.Writer
}

// wireAuthMethodDescriptor is a minimal transient projection of an advertised
// ACP auth method. It preserves the wire id/type discriminator, which the
// pinned SDK union cannot: unknown types must fail closed rather than decode
// as agent methods.
type wireAuthMethodDescriptor struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// wireInitializeAuthResponse reads one initialize response with raw auth
// methods. Agent capabilities reuse the SDK truth; auth descriptors stay out
// of durable Caps snapshots.
type wireInitializeAuthResponse struct {
	AgentCapabilities acpsdk.AgentCapabilities   `json:"agentCapabilities"`
	AuthMethods       []wireAuthMethodDescriptor `json:"authMethods"`
}

func runAuthSession(ctx context.Context, params authSessionParams) (err error) {
	if ctx == nil {
		return errors.New("acp: authentication context is required")
	}
	driver := New(WithControlTimeout(authproviders.DefaultProviderAuthCommandTimeout))
	if params.output != nil {
		sinkLauncher := newLocalLauncher(driver.logger, driver.stopTimeout, 0)
		sinkLauncher.stderrSink = params.output
		driver.launcher = sinkLauncher
	}
	normalized, err := normalizeAuthStartOpts(params.inspection)
	if err != nil {
		return err
	}
	agentName := strings.TrimSpace(normalized.AgentName)
	proc, err := driver.launchAgentProcess(ctx, normalized)
	if err != nil {
		return WrapFailure(store.FailureStartup, "agent subprocess startup failed", err)
	}
	defer func() {
		// Stop owns its separate bounded budget on a background lifetime so
		// caller cancellation cannot strand the disposable process.
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*defaultStopTimeout)
		defer cancel()
		if stopErr := driver.stop(stopCtx, proc, true); stopErr != nil {
			err = errors.Join(err, fmt.Errorf("acp: stop authentication session: %w", stopErr))
		}
	}()

	methods, err := initializeAuthSession(ctx, proc, agentName)
	if err != nil {
		return err
	}
	if err := createAuthProbeSession(ctx, proc, agentName); err == nil {
		return nil
	} else if !params.authenticate || !IsAuthRequiredError(err) {
		return err
	}
	if err := selectAuthMethod(methods, params.methodID); err != nil {
		return err
	}
	if err := sendAuthenticate(ctx, proc, agentName, params.methodID); err != nil {
		return err
	}
	// The empty authenticate result alone never reports authenticated: one
	// readiness session proves the login took effect.
	err = createAuthProbeSession(ctx, proc, agentName)
	return err
}

// normalizeAuthStartOpts resolves the caller's command through the existing
// launch policy without tools, MCP servers, or a hosted tool surface.
func normalizeAuthStartOpts(req SessionInspectionRequest) (StartOpts, error) {
	return normalizeStartOpts(StartOpts{
		AgentName:  strings.TrimSpace(req.AgentName),
		Command:    strings.TrimSpace(req.Command),
		Cwd:        strings.TrimSpace(req.Cwd),
		Env:        append([]string(nil), req.Env...),
		Inspection: true,
		ToolHost:   authRefusingToolHost{},
	})
}

// initializeAuthSession completes one bounded initialize with no advertised
// tool surface and returns the raw advertised auth methods.
func initializeAuthSession(
	ctx context.Context,
	proc *AgentProcess,
	agentName string,
) ([]wireAuthMethodDescriptor, error) {
	initRequest := acpsdk.InitializeRequest{
		ProtocolVersion:    acpsdk.ProtocolVersionNumber,
		ClientCapabilities: acpsdk.ClientCapabilities{},
		ClientInfo: &acpsdk.Implementation{
			Name:    defaultClientName,
			Version: defaultClientVersion,
		},
	}
	response, err := proc.sendControlRequest[wireInitializeAuthResponse](
		ctx,
		acpsdk.AgentMethodInitialize,
		initRequest,
	)
	if err != nil {
		return nil, WrapFailure(
			store.FailureHandshake,
			"ACP initialize handshake failed",
			fmt.Errorf("acp: initialize authentication session for %q: %w", agentName, err),
		)
	}
	proc.setCaps(captureCaps(Caps{
		SteerCapability:       captureSteerCapability(nil, proc.CapsSnapshot().SteerCapability),
		SupportsLoadSession:   response.AgentCapabilities.LoadSession,
		SupportsCloseSession:  response.AgentCapabilities.SessionCapabilities.Close != nil,
		SupportsForkSession:   response.AgentCapabilities.SessionCapabilities.Fork != nil,
		SupportsResumeSession: response.AgentCapabilities.SessionCapabilities.Resume != nil,
		PromptImage:           response.AgentCapabilities.PromptCapabilities.Image,
		PromptAudio:           response.AgentCapabilities.PromptCapabilities.Audio,
		PromptEmbeddedContext: response.AgentCapabilities.PromptCapabilities.EmbeddedContext,
	}, nil, nil))
	return response.AuthMethods, nil
}

// createAuthProbeSession creates one promptless disposable session on the
// existing connection. A nil error means the account is ready; an
// auth-required refusal means it is not. MCP servers stay an empty list (not
// null) so strict agents accept the probe; no hosted tools are attached.
func createAuthProbeSession(
	ctx context.Context,
	proc *AgentProcess,
	agentName string,
) error {
	response, err := proc.sendControlRequest[wireSessionSetupResponse](
		ctx,
		acpsdk.AgentMethodSessionNew,
		wireNewSessionRequest{Cwd: proc.Cwd, McpServers: toSDKMCPServers(nil)},
	)
	if err != nil {
		return WrapFailure(
			store.FailureProtocol,
			"ACP session/new failed",
			fmt.Errorf("acp: create authentication session for %q: %w", agentName, err),
		)
	}
	proc.SessionID = string(response.SessionID)
	proc.bindSessionRoute(response.SessionID)
	return nil
}

// selectAuthMethod validates that the exact configured method is advertised as
// an agent method: omitted type or type "agent", compared byte-for-byte.
func selectAuthMethod(methods []wireAuthMethodDescriptor, methodID string) error {
	for _, method := range methods {
		if method.ID != methodID {
			continue
		}
		switch method.Type {
		case "", "agent":
			return nil
		}
	}
	for _, method := range methods {
		if method.ID != methodID {
			continue
		}
		if method.Type == "terminal" {
			return &AuthMethodUnavailableError{MethodID: methodID, Reason: AuthMethodUnavailableTerminal}
		}
		return &AuthMethodUnavailableError{MethodID: methodID, Reason: AuthMethodUnavailableUnknown}
	}
	return &AuthMethodUnavailableError{MethodID: methodID, Reason: AuthMethodUnavailableAbsent}
}

// sendAuthenticate issues one authenticate call on the caller's login
// lifetime: no short control budget bounds the operator's consent window.
func sendAuthenticate(
	ctx context.Context,
	proc *AgentProcess,
	agentName string,
	methodID string,
) error {
	_, err := acpsdk.SendRequest[acpsdk.AuthenticateResponse](
		proc.conn,
		ctx,
		acpsdk.AgentMethodAuthenticate,
		acpsdk.AuthenticateRequest{MethodId: methodID},
	)
	if err != nil {
		return WrapFailure(
			store.FailureProtocol,
			"ACP authenticate failed",
			fmt.Errorf(
				"acp: authenticate session for %q: %w",
				agentName,
				errors.Join(ctx.Err(), err),
			),
		)
	}
	return nil
}

// authRefusingToolHost refuses every inbound file, terminal, and permission
// operation for disposable authentication processes.
type authRefusingToolHost struct{}

var _ ToolHost = authRefusingToolHost{}

func (authRefusingToolHost) ReadTextFile(_ context.Context, _ string) (string, error) {
	return "", errAuthInboundRefused
}

func (authRefusingToolHost) WriteTextFile(_ context.Context, _ string, _ string) error {
	return errAuthInboundRefused
}

func (authRefusingToolHost) ResolvePath(_ string) (string, error) {
	return "", errAuthInboundRefused
}

func (authRefusingToolHost) Authorize(_ PermissionOperation) error {
	return errAuthInboundRefused
}

func (authRefusingToolHost) PermissionDecision(
	_ acpsdk.RequestPermissionRequest,
) (PermissionDecision, bool) {
	return decisionRejectOnce, false
}

func (authRefusingToolHost) CreateTerminal(
	_ context.Context,
	_ acpsdk.CreateTerminalRequest,
) (acpsdk.CreateTerminalResponse, error) {
	return acpsdk.CreateTerminalResponse{}, errAuthInboundRefused
}

func (authRefusingToolHost) KillTerminal(_ string) error {
	return errAuthInboundRefused
}

func (authRefusingToolHost) TerminalOutput(_ string) (string, error) {
	return "", errAuthInboundRefused
}

func (authRefusingToolHost) WaitForTerminalExit(_ context.Context, _ string) (int, error) {
	return 0, errAuthInboundRefused
}

func (authRefusingToolHost) ReleaseTerminal(_ string) error {
	return errAuthInboundRefused
}
