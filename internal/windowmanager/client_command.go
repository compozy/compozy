package windowmanager

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	clientCommandBuffer  = 16
	clientCommandTimeout = 10 * time.Second
)

type clientCommandEndpoint struct {
	manager     *Manager
	workspaceID WorkspaceID
	clientID    ClientID
	outbound    chan ClientCommand
	done        chan struct{}

	mu      sync.Mutex
	pending map[string]chan ClientCommandResponse
	closed  bool
	err     error
	once    sync.Once
}

var _ ClientCommandConnection = (*clientCommandEndpoint)(nil)

func newClientCommandEndpoint(manager *Manager, workspaceID WorkspaceID, clientID ClientID) *clientCommandEndpoint {
	return &clientCommandEndpoint{
		manager: manager, workspaceID: workspaceID, clientID: clientID,
		outbound: make(chan ClientCommand, clientCommandBuffer),
		done:     make(chan struct{}),
		pending:  make(map[string]chan ClientCommandResponse),
	}
}

func (e *clientCommandEndpoint) Commands() <-chan ClientCommand { return e.outbound }
func (e *clientCommandEndpoint) Done() <-chan struct{}          { return e.done }
func (e *clientCommandEndpoint) Close() error {
	e.closeWithError(ErrClientDisconnected)
	return nil
}

func (e *clientCommandEndpoint) Err() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.err
}

func (e *clientCommandEndpoint) Resolve(response ClientCommandResponse) error {
	if err := validateClientCommandResponse(response); err != nil {
		return err
	}
	e.mu.Lock()
	responses := e.pending[response.CommandID]
	closed := e.closed
	e.mu.Unlock()
	if closed {
		return ErrClientDisconnected
	}
	if responses == nil {
		return fmt.Errorf("client command %q is not pending: %w", response.CommandID, ErrInvalidCommand)
	}
	select {
	case responses <- cloneClientCommandResponse(response):
		return nil
	default:
		return fmt.Errorf("client command %q response overflow: %w", response.CommandID, ErrInvalidCommand)
	}
}

func (e *clientCommandEndpoint) dispatch(
	ctx context.Context,
	command ClientCommand,
) (ClientCommandResponse, error) {
	responses := make(chan ClientCommandResponse, 2)
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return ClientCommandResponse{}, ErrClientDisconnected
	}
	if _, exists := e.pending[command.CommandID]; exists {
		e.mu.Unlock()
		return ClientCommandResponse{}, fmt.Errorf(
			"client command %q already pending: %w",
			command.CommandID,
			ErrInvalidCommand,
		)
	}
	e.pending[command.CommandID] = responses
	e.mu.Unlock()
	defer e.removePending(command.CommandID)

	timer := time.NewTimer(clientCommandTimeout)
	defer timer.Stop()
	select {
	case e.outbound <- cloneClientCommand(command):
	case <-e.done:
		return ClientCommandResponse{}, ErrClientDisconnected
	case <-ctx.Done():
		return ClientCommandResponse{}, ctx.Err()
	case <-timer.C:
		return ClientCommandResponse{}, ErrClientCommandTimedOut
	}
	for {
		select {
		case response := <-responses:
			if response.Status == ClientCommandAcknowledged {
				continue
			}
			if response.Status == ClientCommandFailed {
				return response, fmt.Errorf("client command %q failed: %s", command.CommandID, response.Error)
			}
			return response, nil
		case <-e.done:
			return ClientCommandResponse{}, ErrClientDisconnected
		case <-ctx.Done():
			return ClientCommandResponse{}, ctx.Err()
		case <-timer.C:
			return ClientCommandResponse{}, ErrClientCommandTimedOut
		}
	}
}

func (e *clientCommandEndpoint) removePending(commandID string) {
	e.mu.Lock()
	delete(e.pending, commandID)
	e.mu.Unlock()
}

func (e *clientCommandEndpoint) closeWithError(reason error) {
	e.once.Do(func() {
		e.mu.Lock()
		e.closed = true
		e.err = reason
		e.mu.Unlock()
		close(e.done)
		if e.manager != nil {
			e.manager.removeCommandEndpoint(e)
		}
	})
}

// AttachClientCommands binds one active command channel to an already registered client.
func (m *Manager) AttachClientCommands(
	ctx context.Context,
	workspaceID WorkspaceID,
	clientID ClientID,
) (ClientCommandConnection, error) {
	if err := m.resolveWorkspace(ctx, workspaceID); err != nil {
		return nil, err
	}
	m.mu.Lock()
	if _, exists := m.clients[workspaceID][clientID]; !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("client %q: %w", clientID, ErrClientNotFound)
	}
	endpoint := newClientCommandEndpoint(m, workspaceID, clientID)
	workspaceEndpoints := m.commandEndpoints[workspaceID]
	if workspaceEndpoints == nil {
		workspaceEndpoints = make(map[ClientID]*clientCommandEndpoint)
		m.commandEndpoints[workspaceID] = workspaceEndpoints
	}
	previous := workspaceEndpoints[clientID]
	workspaceEndpoints[clientID] = endpoint
	m.mu.Unlock()
	if previous != nil {
		previous.closeWithError(ErrClientDisconnected)
	}
	context.AfterFunc(ctx, func() { endpoint.closeWithError(ctx.Err()) })
	return endpoint, nil
}

// DispatchClientCommand sends one correlated command and waits for its terminal response.
func (m *Manager) DispatchClientCommand(
	ctx context.Context,
	workspaceID WorkspaceID,
	clientID ClientID,
	command ClientCommand,
) (ClientCommandResponse, error) {
	if err := validateClientCommand(command); err != nil {
		return ClientCommandResponse{}, err
	}
	if err := m.resolveWorkspace(ctx, workspaceID); err != nil {
		return ClientCommandResponse{}, err
	}
	m.mu.Lock()
	endpoint := m.commandEndpoints[workspaceID][clientID]
	m.mu.Unlock()
	if endpoint == nil {
		return ClientCommandResponse{}, ErrClientDisconnected
	}
	return endpoint.dispatch(ctx, command)
}

func (m *Manager) removeCommandEndpoint(endpoint *clientCommandEndpoint) {
	m.mu.Lock()
	workspaceEndpoints := m.commandEndpoints[endpoint.workspaceID]
	if workspaceEndpoints[endpoint.clientID] == endpoint {
		delete(workspaceEndpoints, endpoint.clientID)
	}
	m.mu.Unlock()
}

func (m *Manager) takeCommandEndpointsLocked(workspaceID WorkspaceID) []*clientCommandEndpoint {
	workspaceEndpoints := m.commandEndpoints[workspaceID]
	delete(m.commandEndpoints, workspaceID)
	endpoints := make([]*clientCommandEndpoint, 0, len(workspaceEndpoints))
	for _, endpoint := range workspaceEndpoints {
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}

func closeClientCommandEndpoints(endpoints []*clientCommandEndpoint, reason error) {
	for _, endpoint := range endpoints {
		endpoint.closeWithError(reason)
	}
}

func validateClientCommand(command ClientCommand) error {
	if strings.TrimSpace(command.CommandID) == "" || strings.TrimSpace(command.Op) == "" {
		return fmt.Errorf("client command id and op are required: %w", ErrInvalidCommand)
	}
	return nil
}

func validateClientCommandResponse(response ClientCommandResponse) error {
	if strings.TrimSpace(response.CommandID) == "" {
		return fmt.Errorf("client command response id is required: %w", ErrInvalidCommand)
	}
	switch response.Status {
	case ClientCommandAcknowledged:
		if len(response.Result) != 0 || response.Error != "" {
			return fmt.Errorf("client command ack cannot carry a result or error: %w", ErrInvalidCommand)
		}
	case ClientCommandCompleted:
		if response.Error != "" {
			return fmt.Errorf("client command result cannot carry an error: %w", ErrInvalidCommand)
		}
	case ClientCommandFailed:
		if strings.TrimSpace(response.Error) == "" || len(response.Result) != 0 {
			return fmt.Errorf("client command error requires only an error message: %w", ErrInvalidCommand)
		}
	default:
		return fmt.Errorf("unknown client command response status %q: %w", response.Status, ErrInvalidCommand)
	}
	return nil
}

func cloneClientCommand(command ClientCommand) ClientCommand {
	command.Payload = append([]byte(nil), command.Payload...)
	return command
}

func cloneClientCommandResponse(response ClientCommandResponse) ClientCommandResponse {
	response.Result = append([]byte(nil), response.Result...)
	return response
}
