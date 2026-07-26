package acp

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/toolruntime"
)

// AgentProcess represents one running ACP-backed agent subprocess.
type AgentProcess struct {
	PID       int
	AgentName string
	Command   string
	Args      []string
	Cwd       string
	SessionID string
	Caps      Caps
	StartedAt time.Time

	capsMu          sync.RWMutex
	managed         *subprocess.Process
	handle          sandbox.Handle
	toolHostMu      sync.Mutex
	toolHost        sandbox.ToolHost
	toolGateway     ToolExecutionGateway
	cmd             *exec.Cmd
	conn            *acpsdk.Connection
	stderr          *lockedBuffer
	processCtx      context.Context
	cancelProcess   context.CancelFunc
	permissions     permissionPolicy
	terminals       *terminalManager
	processRegistry *toolruntime.Registry
	processRecord   *toolruntime.Handle
	steerSource     SteerSource

	terminalOwnershipMu sync.RWMutex
	terminalOwnership   map[string]terminalOwnership
	terminalProcessMu   sync.Mutex
	terminalProcesses   map[string]*toolruntime.Handle
	toolPrecheckMu      sync.Mutex
	toolPrechecks       []providerNativeToolPrecheck

	waitMu        sync.RWMutex
	waitErr       error
	done          chan struct{}
	stopRequested bool
	stopMu        sync.RWMutex

	promptMu     sync.RWMutex
	activePrompt *activePromptState

	pendingPermissionMu  sync.Mutex
	pendingPermissions   map[string]*pendingPermission
	permissionRequestSeq uint64
	permissionTimeout    time.Duration

	systemPromptMu       sync.Mutex
	systemPrompt         string
	systemPromptSent     bool
	systemPromptDelivery SystemPromptDeliveryMode
	promptCacheControl   *promptCacheControl

	turnSourceProviderMu sync.RWMutex
	turnSourceProvider   func() string
}

type pendingPermission struct {
	requestID          string
	turnID             string
	response           chan permissionDecision
	supportedDecisions map[permissionDecision]struct{}
}

// Done returns a channel that closes when the subprocess exits.
func (p *AgentProcess) Done() <-chan struct{} {
	return p.done
}

// Wait blocks until the subprocess exits and returns its final error state.
func (p *AgentProcess) Wait() error {
	<-p.Done()
	p.waitMu.RLock()
	defer p.waitMu.RUnlock()
	return p.waitErr
}

// Stderr returns the currently captured stderr output for the subprocess.
func (p *AgentProcess) Stderr() string {
	if p.handle != nil {
		return p.handle.Stderr()
	}
	if p.managed != nil {
		return p.managed.Stderr()
	}
	if p.stderr == nil {
		return ""
	}
	return p.stderr.String()
}

// HealthState returns the latest managed subprocess health snapshot.
func (p *AgentProcess) HealthState() subprocess.HealthState {
	if p == nil || p.managed == nil {
		return subprocess.HealthState{}
	}
	return p.managed.HealthState()
}

// CapsSnapshot returns the latest ACP capability/config snapshot.
func (p *AgentProcess) CapsSnapshot() Caps {
	if p == nil {
		return Caps{}
	}
	p.capsMu.RLock()
	defer p.capsMu.RUnlock()
	return CloneCaps(p.Caps)
}

func (p *AgentProcess) setCaps(caps Caps) {
	if p == nil {
		return
	}
	p.capsMu.Lock()
	defer p.capsMu.Unlock()
	p.Caps = CloneCaps(caps)
}

func (p *AgentProcess) setConfigOptions(options []SessionConfigOption) {
	if p == nil {
		return
	}
	p.capsMu.Lock()
	defer p.capsMu.Unlock()
	p.Caps.ConfigOptions = CloneSessionConfigOptions(options)
}

// ToolHost returns the sandbox-owned tool host used by this process.
func (p *AgentProcess) ToolHost() sandbox.ToolHost {
	if p == nil {
		return nil
	}
	p.toolHostMu.Lock()
	defer p.toolHostMu.Unlock()
	return p.toolHost
}

func (p *AgentProcess) setWaitError(err error) {
	p.waitMu.Lock()
	defer p.waitMu.Unlock()
	p.waitErr = err
}

func (p *AgentProcess) stopWasRequested() bool {
	p.stopMu.RLock()
	defer p.stopMu.RUnlock()
	return p.stopRequested
}

func (p *AgentProcess) markStopRequested() {
	p.stopMu.Lock()
	defer p.stopMu.Unlock()
	p.stopRequested = true
}

func (p *AgentProcess) checkpointProcessOwner(ctx context.Context) error {
	if p == nil || p.processRecord == nil {
		return nil
	}
	if err := p.processRecord.Checkpoint(ctx, toolruntime.ProcessCheckpoint{
		Owner: &toolruntime.ProcessOwner{
			SessionID: p.SessionID,
		},
	}); err != nil {
		return fmt.Errorf("acp: checkpoint process owner: %w", err)
	}
	return nil
}
