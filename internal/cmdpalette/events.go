package cmdpalette

import (
	"context"
	"errors"
	"sync"
	"time"
)

type EventName string

const (
	EventCatalogChanged                 EventName = "cmd_palette.catalog.changed"
	EventCommandInvoked                 EventName = "cmd_palette.command.invoked"
	EventPinChanged                     EventName = "cmd_palette.pin.changed"
	EventBindingChanged                 EventName = "cmd_palette.binding.changed"
	EventAliasChanged                   EventName = "cmd_palette.alias.changed"
	EventPersonalizationReset           EventName = "cmd_palette.personalization.reset"
	EventViewSessionOpened              EventName = "cmd_palette.view_session.opened"
	EventViewSessionClosed              EventName = "cmd_palette.view_session.closed"
	EventViewSessionDegraded            EventName = "cmd_palette.view_session.degraded"
	EventViewSessionCircuitBroken       EventName = "cmd_palette.view_session.circuit_broken"
	EventGlobalHotkeyRegistrationFailed EventName = "global_hotkey.registration_failed"
	eventSubscriberBufferSize                     = 32
)

type Event struct {
	Name            EventName   `json:"-"`
	ProfileLens     ProfileLens `json:"profile_lens"`
	WorkspaceID     WorkspaceID `json:"workspace"`
	CatalogRevision string      `json:"revision,omitempty"`
	CommandID       CommandID   `json:"command_id,omitempty"`
	Pinned          *bool       `json:"pinned,omitempty"`
	Source          string      `json:"source,omitempty"`
	ExecutionSite   ActionKind  `json:"exec_site,omitempty"`
	Outcome         string      `json:"outcome,omitempty"`
	DurationMS      int64       `json:"duration_ms"`
	InvocationID    string      `json:"invocation_id,omitempty"`
	ApprovalID      string      `json:"approval_id,omitempty"`
	ClientID        ClientID    `json:"client_id,omitempty"`
	ViewID          string      `json:"view,omitempty"`
	Extension       string      `json:"extension,omitempty"`
	ViewSessionID   string      `json:"view_session,omitempty"`
	Chord           string      `json:"chord,omitempty"`
	Reason          string      `json:"reason,omitempty"`
	OccurredAt      time.Time   `json:"occurred_at"`
}

// NotifyBindingChanged records one effective shortcut mutation.
func (s *Service) NotifyBindingChanged(
	ctx context.Context,
	profileLens ProfileLens,
	workspaceID WorkspaceID,
	commandID CommandID,
) {
	s.emit(ctx, Event{
		Name: EventBindingChanged, ProfileLens: profileLens,
		WorkspaceID: workspaceID, CommandID: commandID,
	})
}

// NotifyAliasChanged records one effective alias mutation.
func (s *Service) NotifyAliasChanged(
	ctx context.Context,
	profileLens ProfileLens,
	workspaceID WorkspaceID,
	commandID CommandID,
) {
	s.emit(ctx, Event{
		Name: EventAliasChanged, ProfileLens: profileLens,
		WorkspaceID: workspaceID, CommandID: commandID,
	})
}

func (s *Service) emitViewSessionEvent(ctx context.Context, name EventName, session *viewSession) {
	if session == nil {
		return
	}
	s.emit(ctx, Event{
		Name: name, ProfileLens: session.profileLens,
		WorkspaceID: session.workspace, ViewID: session.view,
		Extension: session.extension, ClientID: session.client, ViewSessionID: session.id,
	})
}

// NotifyGlobalHotkeyRegistrationFailed records one shell registration failure.
func (s *Service) NotifyGlobalHotkeyRegistrationFailed(
	ctx context.Context,
	profileLens ProfileLens,
	workspaceID WorkspaceID,
	clientID ClientID,
	commandID CommandID,
	chord string,
	reason string,
) {
	s.emit(ctx, Event{
		Name:        EventGlobalHotkeyRegistrationFailed,
		ProfileLens: profileLens,
		WorkspaceID: workspaceID,
		ClientID:    clientID,
		CommandID:   commandID,
		Chord:       chord,
		Reason:      reason,
	})
}

type EventRecorder interface {
	RecordCmdPaletteEvent(context.Context, Event)
}

type EventSubscriber interface {
	SubscribeCmdPaletteEvents(context.Context, ProfileLens, WorkspaceID) (<-chan Event, func(), error)
}

type ApprovalCompletionReader interface {
	ApprovalCompletionStatus(context.Context, string) (string, error)
}

func (s *Service) SubscribeCmdPaletteEvents(
	ctx context.Context,
	profileLens ProfileLens,
	workspaceID WorkspaceID,
) (<-chan Event, func(), error) {
	if ctx == nil {
		return nil, nil, errors.New("cmd palette: event subscription context is required")
	}
	if workspaceID == "" {
		return nil, nil, errors.New("cmd palette: event subscription workspace is required")
	}
	if err := profileLens.Validate(); err != nil {
		return nil, nil, err
	}
	updates := make(chan Event, eventSubscriberBufferSize)
	s.eventMu.Lock()
	s.nextSubscriber++
	id := s.nextSubscriber
	s.eventSubscribers[id] = eventSubscription{
		profileLens: profileLens,
		workspaceID: workspaceID,
		updates:     updates,
	}
	s.eventMu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			s.eventMu.Lock()
			delete(s.eventSubscribers, id)
			close(updates)
			s.eventMu.Unlock()
		})
	}
	return updates, cancel, nil
}

func (s *Service) NotifyCatalogChanged(
	ctx context.Context,
	profileLens ProfileLens,
	workspaceID WorkspaceID,
) error {
	catalog, err := s.Catalog(ctx, CatalogRequest{
		ProfileLens: profileLens,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return err
	}
	s.emit(ctx, Event{
		Name: EventCatalogChanged, ProfileLens: profileLens, WorkspaceID: workspaceID,
		CatalogRevision: catalog.Revision, OccurredAt: s.now().UTC(),
	})
	return nil
}

func (s *Service) emit(ctx context.Context, event Event) {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = s.now().UTC()
	}
	if s.eventRecorder != nil {
		s.eventRecorder.RecordCmdPaletteEvent(ctx, event)
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	for _, subscriber := range s.eventSubscribers {
		if subscriber.workspaceID != event.WorkspaceID {
			continue
		}
		if !subscriber.profileLens.IsAggregate() && !event.ProfileLens.IsAggregate() &&
			subscriber.profileLens.ID != event.ProfileLens.ID {
			continue
		}
		select {
		case subscriber.updates <- event:
		default:
		}
	}
}

type eventSubscription struct {
	profileLens ProfileLens
	workspaceID WorkspaceID
	updates     chan Event
}
