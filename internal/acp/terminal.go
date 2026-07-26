package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	exec "os/exec"

	"strings"
	"sync"
	"sync/atomic"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/agh/internal/toolruntime"
)

const (
	terminalStatusKey  = "status"
	terminalWindowsKey = "windows"
)

const (
	defaultTerminalOutputLimit = 64 * 1024
	networkCommandName         = "network"
)

type terminalManager struct {
	ctx      context.Context
	logger   *slog.Logger
	registry *toolruntime.Registry

	nextID atomic.Uint64

	mu        sync.RWMutex
	terminals map[string]*managedTerminal
}

type managedTerminal struct {
	id string

	cmd           *exec.Cmd
	processRecord *toolruntime.Handle
	outputLimit   int

	networkOwned   bool
	ownerSessionID string
	ownerTurnID    string

	mu         sync.RWMutex
	output     []byte
	truncated  bool
	exitStatus *acpsdk.TerminalExitStatus
	done       chan struct{}
}

type terminalOwnership struct {
	networkOwned   bool
	ownerSessionID string
	ownerTurnID    string
}

type terminalOutputWriter struct {
	terminal *managedTerminal
}

func (p *AgentProcess) handleCreateTerminal(
	ctx context.Context,
	request acpsdk.CreateTerminalRequest,
) (acpsdk.CreateTerminalResponse, error) {
	request, err := p.interceptCreateTerminalRequest(ctx, request)
	if err != nil {
		return acpsdk.CreateTerminalResponse{}, err
	}

	ownership := terminalOwnership{
		ownerSessionID: p.SessionID,
		ownerTurnID:    p.activeTurnID(),
	}
	if p.isNetworkTurn() {
		argv, err := terminalArgv(request)
		if err != nil {
			return acpsdk.CreateTerminalResponse{}, fmt.Errorf("%w: %s", ErrToolBlockedForNetworkTurn, err)
		}
		if !isAllowedNetworkTerminalArgv(argv) {
			return acpsdk.CreateTerminalResponse{}, ErrToolBlockedForNetworkTurn
		}
		ownership = terminalOwnership{
			networkOwned:   true,
			ownerSessionID: p.SessionID,
			ownerTurnID:    p.activeTurnID(),
		}
	}

	host := p.toolHostOrDefault()
	if localHost, ok := host.(*localToolHost); ok {
		return localHost.createTerminal(ctx, request, ownership)
	}
	response, err := host.CreateTerminal(ctx, request)
	if err != nil {
		return acpsdk.CreateTerminalResponse{}, err
	}
	p.recordTerminalOwnership(response.TerminalId, ownership)
	if err := p.registerExternalTerminalProcess(ctx, host, response.TerminalId, request, ownership); err != nil {
		p.deleteTerminalOwnership(response.TerminalId)
		if killErr := host.KillTerminal(response.TerminalId); killErr != nil {
			slog.Default().Warn(
				"acp: cleanup unregistered terminal",
				"terminal_id", response.TerminalId,
				"error", killErr,
			)
		}
		return acpsdk.CreateTerminalResponse{}, err
	}
	return response, nil
}

func (p *AgentProcess) recordTerminalOwnership(id string, ownership terminalOwnership) {
	if strings.TrimSpace(id) == "" || !ownership.networkOwned {
		return
	}

	p.terminalOwnershipMu.Lock()
	defer p.terminalOwnershipMu.Unlock()
	if p.terminalOwnership == nil {
		p.terminalOwnership = make(map[string]terminalOwnership)
	}
	p.terminalOwnership[id] = ownership
}

func (p *AgentProcess) deleteTerminalOwnership(id string) {
	if strings.TrimSpace(id) == "" {
		return
	}

	p.terminalOwnershipMu.Lock()
	defer p.terminalOwnershipMu.Unlock()
	if p.terminalOwnership == nil {
		return
	}
	delete(p.terminalOwnership, id)
}

func (p *AgentProcess) registerExternalTerminalProcess(
	ctx context.Context,
	host ToolHost,
	id string,
	request acpsdk.CreateTerminalRequest,
	ownership terminalOwnership,
) error {
	if p.processRegistry == nil || strings.TrimSpace(id) == "" {
		return nil
	}
	argv, err := terminalArgv(request)
	if err != nil {
		return err
	}
	registerCtx, cancel := withoutCancelPreservingDeadline(ctx)
	defer cancel()
	cwd := p.Cwd
	if request.Cwd != nil {
		cwd = *request.Cwd
	}
	var handle *toolruntime.Handle
	handle, err = p.processRegistry.Register(registerCtx, toolruntime.RegisterConfig{
		Source: toolruntime.ProcessSourceSandboxTerminal,
		Owner: toolruntime.ProcessOwner{
			SessionID:  ownership.ownerSessionID,
			TurnID:     ownership.ownerTurnID,
			TerminalID: id,
		},
		Command: argv[0],
		Args:    argv[1:],
		Cwd:     cwd,
		Interrupt: func(callbackCtx context.Context, _ toolruntime.ProcessRecord) error {
			if killErr := host.KillTerminal(id); killErr != nil {
				return killErr
			}
			if handle != nil {
				return handle.Complete(
					context.WithoutCancel(callbackCtx),
					toolruntime.ProcessCompletion{Err: errors.New("terminal interrupted")},
				)
			}
			return nil
		},
	})
	if err != nil {
		return fmt.Errorf("acp: register terminal process %q: %w", id, err)
	}

	p.terminalProcessMu.Lock()
	defer p.terminalProcessMu.Unlock()
	if p.terminalProcesses == nil {
		p.terminalProcesses = make(map[string]*toolruntime.Handle)
	}
	p.terminalProcesses[id] = handle
	return nil
}

func (p *AgentProcess) completeExternalTerminalProcess(
	ctx context.Context,
	id string,
	completion toolruntime.ProcessCompletion,
) {
	handle := p.takeExternalTerminalProcess(id)
	if handle == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := handle.Complete(ctx, completion); err != nil {
		slog.Default().Warn("acp: complete terminal process record", "terminal_id", id, "error", err)
	}
}

func (p *AgentProcess) takeExternalTerminalProcess(id string) *toolruntime.Handle {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	p.terminalProcessMu.Lock()
	defer p.terminalProcessMu.Unlock()
	if p.terminalProcesses == nil {
		return nil
	}
	handle := p.terminalProcesses[id]
	delete(p.terminalProcesses, id)
	return handle
}

func (p *AgentProcess) handleKillTerminal(
	request acpsdk.KillTerminalRequest,
) (acpsdk.KillTerminalResponse, error) {
	if err := p.ensureNetworkTurnTerminalAccess(request.TerminalId, false); err != nil {
		return acpsdk.KillTerminalResponse{}, err
	}
	if err := p.toolHostOrDefault().KillTerminal(request.TerminalId); err != nil {
		return acpsdk.KillTerminalResponse{}, err
	}
	p.deleteTerminalOwnership(request.TerminalId)
	p.completeExternalTerminalProcess(
		context.Background(),
		request.TerminalId,
		toolruntime.ProcessCompletion{Err: errors.New("terminal killed")},
	)
	return acpsdk.KillTerminalResponse{}, nil
}

func (p *AgentProcess) handleTerminalOutput(
	request acpsdk.TerminalOutputRequest,
) (acpsdk.TerminalOutputResponse, error) {
	if err := p.ensureNetworkTurnTerminalAccess(request.TerminalId, true); err != nil {
		return acpsdk.TerminalOutputResponse{}, err
	}
	host := p.toolHostOrDefault()
	if localHost, ok := host.(*localToolHost); ok {
		output, truncated, exitStatus, err := localHost.terminalOutputStatus(request.TerminalId)
		if err != nil {
			return acpsdk.TerminalOutputResponse{}, err
		}
		return acpsdk.TerminalOutputResponse{
			Output:     output,
			Truncated:  truncated,
			ExitStatus: exitStatus,
		}, nil
	}
	output, err := host.TerminalOutput(request.TerminalId)
	if err != nil {
		return acpsdk.TerminalOutputResponse{}, err
	}
	return acpsdk.TerminalOutputResponse{
		Output: output,
	}, nil
}

func (p *AgentProcess) handleWaitForTerminalExit(
	ctx context.Context,
	request acpsdk.WaitForTerminalExitRequest,
) (acpsdk.WaitForTerminalExitResponse, error) {
	if err := p.ensureNetworkTurnTerminalAccess(request.TerminalId, true); err != nil {
		return acpsdk.WaitForTerminalExitResponse{}, err
	}
	host := p.toolHostOrDefault()
	if localHost, ok := host.(*localToolHost); ok {
		exitStatus, err := localHost.waitForTerminalExitStatus(ctx, request.TerminalId)
		if err != nil {
			return acpsdk.WaitForTerminalExitResponse{}, err
		}
		if exitStatus == nil {
			return acpsdk.WaitForTerminalExitResponse{}, nil
		}
		return acpsdk.WaitForTerminalExitResponse{
			ExitCode: exitStatus.ExitCode,
			Signal:   exitStatus.Signal,
		}, nil
	}
	exitCode, err := host.WaitForTerminalExit(ctx, request.TerminalId)
	if err != nil {
		return acpsdk.WaitForTerminalExitResponse{}, err
	}
	p.completeExternalTerminalProcess(
		context.Background(),
		request.TerminalId,
		toolruntime.ProcessCompletion{ExitCode: new(exitCode)},
	)
	return acpsdk.WaitForTerminalExitResponse{
		ExitCode: new(exitCode),
	}, nil
}

func (p *AgentProcess) handleReleaseTerminal(
	request acpsdk.ReleaseTerminalRequest,
) (acpsdk.ReleaseTerminalResponse, error) {
	if err := p.ensureNetworkTurnTerminalAccess(request.TerminalId, false); err != nil {
		return acpsdk.ReleaseTerminalResponse{}, err
	}
	if err := p.toolHostOrDefault().ReleaseTerminal(request.TerminalId); err != nil {
		return acpsdk.ReleaseTerminalResponse{}, err
	}
	p.deleteTerminalOwnership(request.TerminalId)
	p.completeExternalTerminalProcess(
		context.Background(),
		request.TerminalId,
		toolruntime.ProcessCompletion{Error: "terminal released"},
	)
	return acpsdk.ReleaseTerminalResponse{}, nil
}

func (p *AgentProcess) toolHostOrDefault() ToolHost {
	p.toolHostMu.Lock()
	defer p.toolHostMu.Unlock()

	if p.toolHost != nil {
		return p.toolHost
	}
	procCtx := p.processCtx
	if procCtx == nil {
		procCtx = context.Background()
	}
	host := newLocalToolHostFromPolicy(
		procCtx,
		p.Cwd,
		p.permissions,
		slog.Default(),
		WithLocalProcessRegistry(p.processRegistry),
	)
	if p.terminals != nil {
		host.terminals = p.terminals
	} else {
		p.terminals = host.terminals
	}
	p.toolHost = host
	return host
}
