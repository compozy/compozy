package extensionpkg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/cmdpalette"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

var _ cmdpalette.ViewProgramProvider = (*Manager)(nil)

// OpenProgram calls view/open through the standard capability and method gate.
func (m *Manager) OpenProgram(
	ctx context.Context,
	extensionName string,
	request cmdpalette.ViewOpenRequest,
) (cmdpalette.ViewFrame, uint64, error) {
	process, name, generation, release, err := m.viewProgramProcess(
		ctx,
		request.ProfileLens,
		request.Workspace,
		extensionName,
		extensionprotocol.ExtensionServiceMethodViewOpen,
	)
	if err != nil {
		return cmdpalette.ViewFrame{}, 0, err
	}
	defer release()
	callCtx, cancel := m.viewProgramCallContext(ctx)
	defer cancel()
	var response cmdpalette.ViewFrame
	if err := process.Call(
		callCtx,
		string(extensionprotocol.ExtensionServiceMethodViewOpen),
		request,
		&response,
	); err != nil {
		return cmdpalette.ViewFrame{}, 0, fmt.Errorf("extension: view open via %q: %w", name, err)
	}
	return response, generation, nil
}

// HandleProgramEvent calls view/event and permits an acknowledgement-only response.
func (m *Manager) HandleProgramEvent(
	ctx context.Context,
	profile cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
	extensionName string,
	event cmdpalette.ViewEvent,
) (*cmdpalette.ViewFrame, error) {
	process, name, _, release, err := m.viewProgramProcess(
		ctx,
		profile,
		workspaceID,
		extensionName,
		extensionprotocol.ExtensionServiceMethodViewEvent,
	)
	if err != nil {
		return nil, err
	}
	defer release()
	callCtx, cancel := m.viewProgramCallContext(ctx)
	defer cancel()
	var response json.RawMessage
	if err := process.Call(
		callCtx,
		string(extensionprotocol.ExtensionServiceMethodViewEvent),
		event,
		&response,
	); err != nil {
		return nil, fmt.Errorf("extension: view event via %q: %w", name, err)
	}
	trimmed := bytes.TrimSpace(response)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("{}")) {
		return nil, nil
	}
	var frame cmdpalette.ViewFrame
	if err := json.Unmarshal(trimmed, &frame); err != nil {
		return nil, fmt.Errorf("extension: decode view event via %q: %w", name, err)
	}
	return &frame, nil
}

// CloseProgram calls view/close through the same gate as open and event.
func (m *Manager) CloseProgram(
	ctx context.Context,
	profile cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
	extensionName string,
	request cmdpalette.ViewCloseRequest,
) error {
	process, name, _, release, err := m.viewProgramProcess(
		ctx,
		profile,
		workspaceID,
		extensionName,
		extensionprotocol.ExtensionServiceMethodViewClose,
	)
	if err != nil {
		if errors.Is(err, toolspkg.ErrToolUnavailable) {
			return nil
		}
		return err
	}
	defer release()
	callCtx, cancel := m.viewProgramCallContext(ctx)
	defer cancel()
	var response struct{}
	if err := process.Call(
		callCtx,
		string(extensionprotocol.ExtensionServiceMethodViewClose),
		request,
		&response,
	); err != nil {
		return fmt.Errorf("extension: view close via %q: %w", name, err)
	}
	return nil
}

func (m *Manager) viewProgramProcess(
	ctx context.Context,
	profile cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
	extensionName string,
	method extensionprotocol.ExtensionServiceMethod,
) (processHandle, string, uint64, func(), error) {
	key := ProfileInstanceKey(extensionName, string(profile.ID), string(workspaceID))
	release, err := m.viewCallGates.acquire(ctx, key)
	if err != nil {
		return nil, key.Name, 0, nil, fmt.Errorf("extension: wait for view call slot: %w", err)
	}
	process, name, err := m.extensionServiceProcessForInstance(ctx, key, method)
	if err != nil {
		release()
		return nil, name, 0, nil, err
	}
	m.mu.RLock()
	extension := m.instanceLocked(key)
	if extension == nil && !key.IsGlobal() {
		extension = m.extensions[key.Name]
	}
	if extension == nil || extension.process != process || extension.generation < 0 {
		m.mu.RUnlock()
		release()
		return nil, name, 0, nil, fmt.Errorf(
			"extension: extension %q changed while opening a view: %w",
			name,
			toolspkg.ErrToolUnavailable,
		)
	}
	generation := uint64(extension.generation)
	m.mu.RUnlock()
	return process, name, generation, release, nil
}

func (m *Manager) viewProgramCallContext(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := defaultViewTimeout
	if m != nil && m.defaultViewTimeout > 0 {
		timeout = m.defaultViewTimeout
	}
	return context.WithTimeout(ctx, timeout)
}
