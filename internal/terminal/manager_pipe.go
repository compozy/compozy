package terminal

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
)

// PipeRequest describes a captured, non-interactive command owned by a consumer.
type PipeRequest struct {
	WS           string
	Argv         []string
	Cwd          string
	Env          map[string]string
	Title        string
	AllowedRoots []string
	Actor        Actor
	Capabilities Capabilities
}

// OpenPipe starts a non-interactive command for protocol adapters such as ACP.
func (m *Service) OpenPipe(ctx context.Context, request PipeRequest) (Handle, error) {
	if ctx == nil {
		return nil, errors.New("terminal: open pipe context is required")
	}
	if len(request.Argv) == 0 || strings.TrimSpace(request.Argv[0]) == "" {
		return nil, errors.New("terminal: pipe command is required")
	}
	request.Title = SanitizeTitle(request.Title)
	if err := m.admit(ctx, request.WS, request.Actor); err != nil {
		return nil, err
	}
	cwd, workspaceID, err := m.resolveOpenWorkspace(
		ctx, request.WS, request.Cwd, request.Actor.ProfileID, request.AllowedRoots...,
	)
	if err != nil {
		return nil, err
	}
	settings, err := m.settings(ctx, workspaceID, request.Actor.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("terminal: resolve pipe settings: %w", err)
	}
	if err := validateSettings(settings); err != nil {
		return nil, err
	}
	openRequest := OpenRequest{WS: workspaceID, Actor: request.Actor}
	releaseAdmission, err := m.reserveAdmission(ctx, openRequest, settings)
	if err != nil {
		return nil, err
	}
	defer releaseAdmission()
	id, err := newTerminalID(m.entropy)
	if err != nil {
		return nil, err
	}
	nonce, err := newMarkerNonce(m.entropy)
	if err != nil {
		return nil, err
	}
	spec := ProcSpec{
		Argv: append([]string(nil), request.Argv...), Cwd: cwd, Env: cloneStringMap(request.Env),
		Cols: 80, Rows: 24, Mode: terminalpty.ModePipe, MarkerNonce: nonce,
	}
	proc, err := m.pty.Start(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("terminal: start pipe command %q: %w", request.Argv[0], err)
	}
	info := ownedInfo(Info{
		ID: id, WS: workspaceID, ProfileID: request.Actor.ProfileID,
		Title: request.Title, Shell: request.Argv[0], Cwd: cwd, Mode: ModePipe, State: terminalStateRunning,
		Controller: cloneActor(&request.Actor), Capabilities: request.Capabilities, CreatedAt: m.now(),
	}, request.Actor)
	profileName := m.eventProfileName(ctx, request.Actor.ProfileID)
	item := newSession(ctx, m, proc, info, settings, nonce, profileName, 80, 24, true)
	processRecord, err := m.processRegistration(ctx, item, spec)
	if err != nil {
		return nil, errors.Join(err, cleanupUnregisteredProcess(ctx, proc))
	}
	item.processRecord = processRecord
	key := terminalKey{workspaceID: workspaceID, profileID: request.Actor.ProfileID, id: id}
	if err := m.insert(key, item); err != nil {
		return nil, cleanupRegisteredProcess(ctx, proc, processRecord, err)
	}
	m.registerJournalTerminal(item)
	opened := item.Info()
	m.events.Notify(ctx, Event{
		Kind: EventKindOpened, WorkspaceID: workspaceID, ProfileID: request.Actor.ProfileID,
		ProfileName: profileName,
		TerminalID:  id, Actor: request.Actor, Info: &opened,
		Detail: &EventDetail{Mode: opened.Mode, Cwd: opened.Cwd, Title: opened.Title}, At: m.now(),
	})
	item.start()
	return item, nil
}

// Release closes and immediately forgets a consumer-owned terminal.
func (m *Service) Release(ctx context.Context, workspaceID, profileID string, id ID, actor Actor) error {
	if err := requestContextError(ctx, "release"); err != nil {
		return err
	}
	if actor.ProfileID != profileID {
		return &Error{Code: errorCodeNotFound, Message: errorMessageNotFound, Err: ErrNotFound}
	}
	key := terminalKey{workspaceID: workspaceID, profileID: profileID, id: id}
	item, err := m.lookup(key)
	if err != nil {
		return err
	}
	if _, err := item.close(ctx, SignalHUP, "released", actor); err != nil && !errors.Is(err, ErrExited) {
		return err
	}
	m.removeWithTombstone(key, item, m.tombstoneExpiry(item))
	return nil
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(source))
	maps.Copy(cloned, source)
	return cloned
}
