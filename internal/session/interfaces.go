// Package session orchestrates AGH session lifecycle around ACP-backed agents.
package session

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/sandbox"
	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/toolruntime"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// TurnSource classifies the origin of a prompt turn inside the daemon runtime.
type TurnSource string

const (
	TurnSourceUser      TurnSource = TurnSource(acp.PromptTurnSourceUser)
	TurnSourceNetwork   TurnSource = TurnSource(acp.PromptTurnSourceNetwork)
	TurnSourceSynthetic TurnSource = TurnSource(acp.PromptTurnSourceSynthetic)
)

// PromptOpts carries per-turn metadata through the session prompt pipeline.
type PromptOpts struct {
	Message         string
	TurnSource      TurnSource
	PromptMeta      acp.PromptMeta
	DeliveryContext context.Context
	PrepareDelivery PromptDeliveryPreparer
}

// PromptDelivery identifies a prompt whose agent-event stream is ready to start.
type PromptDelivery struct {
	SessionID string
	TurnID    string
}

// PromptDeliveryPreparer runs after the provider accepts a prompt and before its first event is pumped.
type PromptDeliveryPreparer func(context.Context, PromptDelivery) error

// NetworkPeerCapability is the runtime-owned capability projection shared with
// the network join lifecycle for brief and rich discovery.
type NetworkPeerCapability struct {
	ID                string
	Summary           string
	Outcome           string
	Version           string
	Digest            string
	ContextNeeded     []string
	ArtifactsExpected []string
	ExecutionOutline  []string
	Constraints       []string
	Examples          []string
	Requirements      []string
}

// NetworkPeerJoin describes one daemon-local peer registration request for the
// late-bound network lifecycle.
type NetworkPeerJoin struct {
	SessionID            string
	PeerID               string
	WorkspaceID          string
	DisplayName          string
	Channel              string
	OwnerKey             string
	NetworkParticipation participation.Spec
	Capabilities         []NetworkPeerCapability
}

// NetworkPeerLifecycle is the late-bound network join/leave surface consumed by
// the session manager without importing the network package.
type NetworkPeerLifecycle interface {
	JoinChannel(ctx context.Context, join NetworkPeerJoin) error
	LeaveChannel(ctx context.Context, sessionID string) error
}

// TurnEndNotifier is invoked once after a prompt turn finishes dispatching.
type TurnEndNotifier func(sessionID string)

// PromptInputAugmenter can add bounded daemon-local context before prompt dispatch.
type PromptInputAugmenter func(ctx context.Context, session *Session, message string) (string, error)

// LedgerMaterializer is the thin session-end seam for forensic ledger projection.
type LedgerMaterializer interface {
	MaterializeSessionLedger(ctx context.Context, record store.SessionLedgerRecord) error
	DiscardSessionLedger(ctx context.Context, record store.SessionLedgerRecord) error
}

// AgentArtifacts returns an agent definition and optional resource-backed authored-context sidecars.
type AgentArtifacts struct {
	Agent               aghconfig.AgentDef
	ResourceID          string
	OwnerKind           string
	OwnerID             string
	Scope               resources.ResourceScope
	PackageOwned        bool
	SoulSourcePath      string
	SoulBody            string
	HeartbeatSourcePath string
	HeartbeatBody       string
}

// AgentArtifactResolver resolves agent provenance and sidecars when available.
type AgentArtifactResolver interface {
	ResolveAgentArtifacts(name string, resolved *workspacepkg.ResolvedWorkspace) (AgentArtifacts, error)
}

func normalizeTurnSource(source TurnSource) TurnSource {
	switch TurnSource(strings.TrimSpace(string(source))) {
	case "", TurnSourceUser:
		return TurnSourceUser
	case TurnSourceNetwork:
		return TurnSourceNetwork
	case TurnSourceSynthetic:
		return TurnSourceSynthetic
	default:
		return ""
	}
}

// AgentProcess is the session-owned handle for a running agent process.
type AgentProcess struct {
	PID       int
	AgentName string
	Command   string
	Args      []string
	Cwd       string
	SessionID string
	Caps      acp.Caps
	StartedAt time.Time

	done                <-chan struct{}
	waitFn              func() error
	stderrFn            func() string
	healthStateFn       func() subprocess.HealthState
	capsSnapshotFn      func() acp.Caps
	approvePermissionFn func(context.Context, acp.ApproveRequest) error
	requestPermissionFn func(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error)
	configureRuntimeFn  func(func() TurnSource)
	toolHostFn          func() sandbox.ToolHost
	toolHost            sandbox.ToolHost
	native              any
	waitOverrideMu      sync.RWMutex
	waitErrOverride     error
}

// AgentProcessOptions defines the exported fields and lifecycle hooks needed to construct an AgentProcess.
type AgentProcessOptions struct {
	PID               int
	AgentName         string
	Command           string
	Args              []string
	Cwd               string
	SessionID         string
	Caps              acp.Caps
	StartedAt         time.Time
	Done              <-chan struct{}
	Wait              func() error
	Stderr            func() string
	HealthState       func() subprocess.HealthState
	ApprovePermission func(context.Context, acp.ApproveRequest) error
	RequestPermission func(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error)
	CapsSnapshot      func() acp.Caps
	ConfigureRuntime  func(func() TurnSource)
	ToolHost          sandbox.ToolHost
}

// NewAgentProcess constructs an AgentProcess for custom AgentDriver implementations.
func NewAgentProcess(opts AgentProcessOptions) *AgentProcess {
	done := opts.Done
	if done == nil {
		ch := make(chan struct{})
		close(ch)
		done = ch
	}

	waitFn := opts.Wait
	if waitFn == nil {
		waitFn = func() error {
			<-done
			return nil
		}
	}

	stderrFn := opts.Stderr
	if stderrFn == nil {
		stderrFn = func() string { return "" }
	}

	return &AgentProcess{
		PID:                 opts.PID,
		AgentName:           opts.AgentName,
		Command:             opts.Command,
		Args:                append([]string(nil), opts.Args...),
		Cwd:                 opts.Cwd,
		SessionID:           opts.SessionID,
		Caps:                opts.Caps,
		StartedAt:           opts.StartedAt,
		done:                done,
		waitFn:              waitFn,
		stderrFn:            stderrFn,
		healthStateFn:       opts.HealthState,
		capsSnapshotFn:      opts.CapsSnapshot,
		approvePermissionFn: opts.ApprovePermission,
		requestPermissionFn: opts.RequestPermission,
		configureRuntimeFn:  opts.ConfigureRuntime,
		toolHost:            opts.ToolHost,
	}
}

// Done reports when the underlying runtime process exits.
func (p *AgentProcess) Done() <-chan struct{} {
	return p.done
}

// Wait blocks until the runtime process exits and returns its terminal error state.
func (p *AgentProcess) Wait() error {
	<-p.Done()
	p.waitOverrideMu.RLock()
	override := p.waitErrOverride
	p.waitOverrideMu.RUnlock()
	if override != nil {
		return override
	}
	return p.waitFn()
}

// Stderr returns any captured stderr output for the runtime process.
func (p *AgentProcess) Stderr() string {
	return p.stderrFn()
}

// HealthState returns the latest runtime health snapshot when the driver
// exposes subprocess health monitoring.
func (p *AgentProcess) HealthState() (subprocess.HealthState, bool) {
	if p == nil || p.healthStateFn == nil {
		return subprocess.HealthState{}, false
	}
	return p.healthStateFn(), true
}

// CapsSnapshot returns the latest ACP capability/config snapshot for this process.
func (p *AgentProcess) CapsSnapshot() acp.Caps {
	if p == nil {
		return acp.Caps{}
	}
	if p.capsSnapshotFn != nil {
		return acp.CloneCaps(p.capsSnapshotFn())
	}
	return acp.CloneCaps(p.Caps)
}

// ToolHost returns the sandbox-owned tool host when the process exposes one.
func (p *AgentProcess) ToolHost() sandbox.ToolHost {
	if p == nil {
		return nil
	}
	if p.toolHostFn != nil {
		return p.toolHostFn()
	}
	return p.toolHost
}

// ApprovePermission resolves one pending interactive permission request.
func (p *AgentProcess) ApprovePermission(ctx context.Context, req acp.ApproveRequest) error {
	if p.approvePermissionFn == nil {
		return errors.New("session: permission approval is not supported")
	}
	return p.approvePermissionFn(ctx, req)
}

// RequestPermission asks the active runtime permission path for a tool approval decision.
func (p *AgentProcess) RequestPermission(
	ctx context.Context,
	req acp.RequestPermissionRequest,
) (acp.RequestPermissionResponse, error) {
	if p.requestPermissionFn == nil {
		return acp.RequestPermissionResponse{}, errors.New("session: permission request is not supported")
	}
	return p.requestPermissionFn(ctx, req)
}

func (p *AgentProcess) configureRuntime(currentTurnSource func() TurnSource) {
	if p == nil || p.configureRuntimeFn == nil {
		return
	}
	p.configureRuntimeFn(currentTurnSource)
}

func (p *AgentProcess) setWaitErrorOverride(err error) {
	if p == nil {
		return
	}
	p.waitOverrideMu.Lock()
	defer p.waitOverrideMu.Unlock()
	p.waitErrOverride = err
}

func wrapACPProcess(proc *acp.AgentProcess) *AgentProcess {
	if proc == nil {
		return nil
	}

	return &AgentProcess{
		PID:            proc.PID,
		AgentName:      proc.AgentName,
		Command:        proc.Command,
		Args:           append([]string(nil), proc.Args...),
		Cwd:            proc.Cwd,
		SessionID:      proc.SessionID,
		Caps:           proc.CapsSnapshot(),
		StartedAt:      proc.StartedAt,
		done:           proc.Done(),
		waitFn:         proc.Wait,
		stderrFn:       proc.Stderr,
		healthStateFn:  proc.HealthState,
		capsSnapshotFn: proc.CapsSnapshot,
		approvePermissionFn: func(ctx context.Context, req acp.ApproveRequest) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return proc.ResolvePermission(req)
		},
		requestPermissionFn: func(
			ctx context.Context,
			req acp.RequestPermissionRequest,
		) (acp.RequestPermissionResponse, error) {
			if err := ctx.Err(); err != nil {
				return acp.RequestPermissionResponse{}, err
			}
			return proc.RequestPermission(ctx, req)
		},
		configureRuntimeFn: func(currentTurnSource func() TurnSource) {
			proc.SetTurnSourceProvider(func() string {
				if currentTurnSource == nil {
					return ""
				}
				return string(currentTurnSource())
			})
		},
		toolHostFn: proc.ToolHost,
		native:     proc,
	}
}

// AgentDriver defines the ACP functionality consumed by the session manager.
type AgentDriver interface {
	Start(ctx context.Context, opts acp.StartOpts) (*AgentProcess, error)
	Prompt(ctx context.Context, proc *AgentProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error)
	Cancel(ctx context.Context, proc *AgentProcess) error
	Stop(ctx context.Context, proc *AgentProcess) error
}

// ErrScopedInterruptNotFound reports that no registered tool process matched a scoped interrupt.
var ErrScopedInterruptNotFound = toolruntime.ErrProcessNotFound

// ScopedInterrupter is the optional process-scoped interrupt surface for drivers.
type ScopedInterrupter interface {
	Interrupt(ctx context.Context, sessionID string, turnID string) (toolruntime.InterruptReport, error)
}

// EventRecorder is the per-session storage surface consumed by session/.
type EventRecorder = store.EventRecorder

// EventReadCloser is the read-only per-session storage surface consumed by query paths.
type EventReadCloser = store.EventReadCloser

// Notifier fans out session lifecycle and prompt events to downstream observers.
type Notifier interface {
	OnSessionCreated(ctx context.Context, session *Session)
	OnSessionStopped(ctx context.Context, session *Session)
	OnAgentEvent(ctx context.Context, sessionID string, event any)
}

// FinalizationNotifier is an optional lifecycle seam invoked after the process and sandbox
// stop, but before the recorder closes and the durable ledger is materialized.
type FinalizationNotifier interface {
	OnSessionFinalizing(ctx context.Context, session *Session)
}

// AgentEventNotifier is an optional notifier extension that receives the
// active session alongside streamed agent events.
type AgentEventNotifier interface {
	OnAgentEventForSession(ctx context.Context, session *Session, event any)
}

// PromptAssembler assembles the prompt context for a new session start.
type PromptAssembler interface {
	Assemble(ctx context.Context, agent aghconfig.AgentDef, workspace *workspacepkg.ResolvedWorkspace) (string, error)
}

// AgentResolver resolves agent definitions from the daemon-authoritative catalog.
type AgentResolver interface {
	ResolveAgent(name string, resolved *workspacepkg.ResolvedWorkspace) (aghconfig.AgentDef, error)
}

// SkillRegistry resolves the active skill set for a workspace during session start.
type SkillRegistry interface {
	ForWorkspace(ctx context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*skillspkg.Skill, error)
	ForAgentDef(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agent aghconfig.AgentDef,
	) ([]*skillspkg.Skill, error)
}

// MCPResolver resolves skill-declared MCP servers into runtime config entries.
type MCPResolver interface {
	Resolve(skills []*skillspkg.Skill) []aghconfig.MCPServer
}

// ACPDriverAdapter adapts the concrete ACP driver to the session-local interface.
type ACPDriverAdapter struct {
	driver *acp.Driver
}

var _ AgentDriver = (*ACPDriverAdapter)(nil)
var _ ScopedInterrupter = (*ACPDriverAdapter)(nil)

// NewACPDriverAdapter wraps the provided ACP driver for use by the session manager.
func NewACPDriverAdapter(driver *acp.Driver) *ACPDriverAdapter {
	return &ACPDriverAdapter{driver: driver}
}

// Start launches a new ACP-backed runtime process.
func (a *ACPDriverAdapter) Start(ctx context.Context, opts acp.StartOpts) (*AgentProcess, error) {
	proc, err := a.driver.Start(ctx, opts)
	if err != nil {
		return nil, err
	}
	return wrapACPProcess(proc), nil
}

// Prompt streams prompt events from the wrapped ACP runtime.
func (a *ACPDriverAdapter) Prompt(
	ctx context.Context,
	proc *AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	native, err := a.nativeProcess(proc)
	if err != nil {
		return nil, err
	}
	return a.driver.Prompt(ctx, native, req)
}

// Cancel lets higher layers stop prompt work without bypassing ACP-owned lifecycle handling.
func (a *ACPDriverAdapter) Cancel(ctx context.Context, proc *AgentProcess) error {
	native, err := a.nativeProcess(proc)
	if err != nil {
		return err
	}
	return a.driver.Cancel(ctx, native)
}

// Interrupt signals only registered tool processes scoped to the session turn.
func (a *ACPDriverAdapter) Interrupt(
	ctx context.Context,
	sessionID string,
	turnID string,
) (toolruntime.InterruptReport, error) {
	return a.driver.Interrupt(ctx, toolruntime.InterruptScope{
		SessionID: sessionID,
		TurnID:    turnID,
		Reason:    "prompt canceled",
	})
}

// Stop stops the wrapped ACP runtime process.
func (a *ACPDriverAdapter) Stop(ctx context.Context, proc *AgentProcess) error {
	native, err := a.nativeProcess(proc)
	if err != nil {
		return err
	}
	return a.driver.Stop(ctx, native)
}

func (a *ACPDriverAdapter) nativeProcess(proc *AgentProcess) (*acp.AgentProcess, error) {
	if proc == nil {
		return nil, errors.New("session: agent process is required")
	}

	native, ok := proc.native.(*acp.AgentProcess)
	if !ok || native == nil {
		return nil, errors.New("session: unsupported agent process implementation")
	}
	return native, nil
}
