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
	EventPersonalizationReset           EventName = "cmd_palette.personalization.reset"
	EventGlobalHotkeyRegistrationFailed EventName = "global_hotkey.registration_failed"
	eventSubscriberLimit                          = 32
)

type Event struct {
	Name            EventName   `json:"-"`
	WorkspaceID     WorkspaceID `json:"workspace"`
	CatalogRevision string      `json:"catalog_revision,omitempty"`
	CommandID       CommandID   `json:"command_id,omitempty"`
	Pinned          *bool       `json:"pinned,omitempty"`
	Source          string      `json:"source,omitempty"`
	ExecutionSite   ActionKind  `json:"exec_site,omitempty"`
	Outcome         string      `json:"outcome,omitempty"`
	DurationMS      int64       `json:"duration_ms"`
	InvocationID    string      `json:"invocation_id,omitempty"`
	ApprovalID      string      `json:"approval_id,omitempty"`
	ClientID        ClientID    `json:"client_id,omitempty"`
	Chord           string      `json:"chord,omitempty"`
	Reason          string      `json:"reason,omitempty"`
	OccurredAt      time.Time   `json:"occurred_at"`
}

// NotifyGlobalHotkeyRegistrationFailed records one shell registration failure.
func (s *Service) NotifyGlobalHotkeyRegistrationFailed(
	ctx context.Context,
	workspaceID WorkspaceID,
	clientID ClientID,
	commandID CommandID,
	chord string,
	reason string,
) {
	s.emit(ctx, Event{
		Name:        EventGlobalHotkeyRegistrationFailed,
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
	SubscribeCmdPaletteEvents(context.Context, WorkspaceID) (<-chan Event, func(), error)
}

type ApprovalCompletionReader interface {
	ApprovalCompletionStatus(context.Context, string) (string, error)
}

func (s *Service) SubscribeCmdPaletteEvents(
	ctx context.Context,
	workspaceID WorkspaceID,
) (<-chan Event, func(), error) {
	if ctx == nil {
		return nil, nil, errors.New("cmd palette: event subscription context is required")
	}
	if workspaceID == "" {
		return nil, nil, errors.New("cmd palette: event subscription workspace is required")
	}
	updates := make(chan Event, eventSubscriberLimit)
	s.eventMu.Lock()
	s.nextSubscriber++
	id := s.nextSubscriber
	s.eventSubscribers[id] = eventSubscription{workspaceID: workspaceID, updates: updates}
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

func (s *Service) NotifyCatalogChanged(ctx context.Context, workspaceID WorkspaceID) error {
	catalog, err := s.Catalog(ctx, workspaceID, "")
	if err != nil {
		return err
	}
	s.emit(ctx, Event{
		Name: EventCatalogChanged, WorkspaceID: workspaceID,
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
		select {
		case subscriber.updates <- event:
		default:
		}
	}
}

type eventSubscription struct {
	workspaceID WorkspaceID
	updates     chan Event
}
