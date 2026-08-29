package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func newTerminalManager(
	ctx context.Context,
	logger *slog.Logger,
	core TerminalHost,
	scope LocalTerminalScope,
) *terminalManager {
	if logger == nil {
		logger = slog.Default()
	}
	var coreErr error
	if core == nil {
		coreErr = errors.New("acp: terminal core is required")
	}
	return &terminalManager{
		logger: logger, lifecycle: ctx, core: core, coreErr: coreErr, scope: scope,
		terminals: make(map[string]*managedTerminal),
	}
}

func (m *terminalManager) create(
	ctx context.Context,
	cwd string,
	request acpsdk.CreateTerminalRequest,
	ownership terminalOwnership,
) (acpsdk.CreateTerminalResponse, error) {
	if ctx == nil {
		return acpsdk.CreateTerminalResponse{}, errors.New("acp: create terminal context is required")
	}
	if m != nil && m.coreErr != nil {
		return acpsdk.CreateTerminalResponse{}, m.coreErr
	}
	if m == nil || m.core == nil {
		return acpsdk.CreateTerminalResponse{}, errors.New("acp: terminal core is required")
	}
	argv, err := terminalArgv(request)
	if err != nil {
		return acpsdk.CreateTerminalResponse{}, err
	}
	env := make(map[string]string, len(request.Env))
	if !ownership.networkOwned {
		for _, variable := range request.Env {
			env[variable.Name] = variable.Value
		}
	}
	actor := m.actor(ownership)
	handle, err := m.core.OpenPipe(ctx, terminalpkg.PipeRequest{
		WS: m.scope.WorkspaceID, Argv: argv, Cwd: cwd, Env: env,
		Title: strings.Join(argv, " "), Actor: actor,
		AllowedRoots: append([]string(nil), m.scope.AllowedRoots...),
		Capabilities: terminalpkg.Capabilities{Interactive: false},
	})
	if err != nil {
		return acpsdk.CreateTerminalResponse{}, fmt.Errorf("acp: create terminal: %w", err)
	}
	outputLimit := defaultTerminalOutputLimit
	if request.OutputByteLimit != nil {
		outputLimit = max(*request.OutputByteLimit, 0)
	}
	info := handle.Info()
	managed := &managedTerminal{
		handle: handle, outputLimit: outputLimit, ownership: ownership, actor: actor,
	}
	m.mu.Lock()
	m.terminals[string(info.ID)] = managed
	m.mu.Unlock()
	return acpsdk.CreateTerminalResponse{TerminalId: string(info.ID)}, nil
}

func (m *terminalManager) actor(ownership terminalOwnership) terminalpkg.Actor {
	if ownership.systemOwned {
		return terminalpkg.Actor{
			Kind: terminalpkg.ActorKindSystem, ID: localToolHostActorID, ProfileID: m.scope.ProfileID,
		}
	}
	sessionID := strings.TrimSpace(ownership.ownerSessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(m.scope.SessionID)
	}
	return terminalpkg.Actor{
		Kind: terminalpkg.ActorKindAgent, ID: m.scope.ActorID, ProfileID: m.scope.ProfileID,
		SessionID: sessionID, RunID: strings.TrimSpace(ownership.ownerRunID),
		Generation: ownership.ownerGeneration,
	}
}
