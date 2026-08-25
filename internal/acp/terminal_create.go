package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/toolruntime"
)

func newTerminalManager(
	ctx context.Context,
	logger *slog.Logger,
	root string,
	core TerminalHost,
	scope terminalScope,
	registries ...*toolruntime.Registry,
) *terminalManager {
	if logger == nil {
		logger = slog.Default()
	}
	if strings.TrimSpace(scope.workspaceID) == "" {
		scope.workspaceID = "acp-local:" + root
	}
	if strings.TrimSpace(scope.profileID) == "" {
		scope.profileID = "acp-local"
	}
	if strings.TrimSpace(scope.actorID) == "" {
		scope.actorID = "acp-agent"
	}
	ownedCore := (*terminalpkg.Service)(nil)
	var coreErr error
	if core == nil {
		options := []terminalpkg.Option{terminalpkg.WithLogger(logger)}
		if len(registries) > 0 && registries[0] != nil {
			options = append(options, terminalpkg.WithProcessRegistry(registries[0]))
		}
		service, err := terminalpkg.NewManager(options...)
		if err != nil {
			coreErr = fmt.Errorf("acp: create local terminal core: %w", err)
		} else if err := service.Start(ctx); err != nil {
			coreErr = fmt.Errorf("acp: start local terminal core: %w", err)
		} else {
			core = service
			ownedCore = service
		}
	}
	return &terminalManager{
		ctx: ctx, logger: logger, core: core, ownedCore: ownedCore, coreErr: coreErr, scope: scope,
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
		WS: m.scope.workspaceID, Argv: argv, Cwd: cwd, Env: env,
		Title: strings.Join(argv, " "), Actor: actor,
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
	sessionID := strings.TrimSpace(ownership.ownerSessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(m.scope.sessionID)
	}
	return terminalpkg.Actor{
		Kind: terminalpkg.ActorKindAgent, ID: m.scope.actorID, ProfileID: m.scope.profileID,
		SessionID: sessionID, RunID: strings.TrimSpace(ownership.ownerTurnID),
		Generation: m.scope.generation,
	}
}
