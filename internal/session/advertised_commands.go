package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

func (s *Session) replaceAdvertisedCommands(commands []store.SessionAdvertisedCommand, observedAt time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.AdvertisedCommands = store.CloneSessionAdvertisedCommands(commands)
	if !observedAt.IsZero() && observedAt.After(s.UpdatedAt) {
		s.UpdatedAt = observedAt.UTC()
	}
	s.mu.Unlock()
}

func (m *Manager) persistAdvertisedCommandsFromEvent(
	_ context.Context,
	session *Session,
	event acp.AgentEvent,
) error {
	if event.Type != acp.EventTypeAvailableCommands {
		return nil
	}
	commands := normalizeAdvertisedCommands(event.AvailableCommands.Values())
	for _, command := range commands {
		if err := command.Validate(); err != nil {
			return fmt.Errorf("session: validate advertised command %q: %w", command.Name, err)
		}
	}
	session.replaceAdvertisedCommands(commands, event.Timestamp)
	if err := m.persistSessionMetadataOnly(session); err != nil {
		return fmt.Errorf("session: persist advertised command replacement set: %w", err)
	}
	return nil
}

func (m *Manager) restoreAdvertisedCommands(ctx context.Context, session *Session) error {
	if session == nil {
		return nil
	}
	recorder := session.recorderHandle()
	if recorder == nil {
		return nil
	}
	events, err := recorder.Query(ctx, store.EventQuery{Type: acp.EventTypeAvailableCommands, Limit: 1})
	if err != nil {
		return fmt.Errorf("session: restore advertised commands: %w", err)
	}
	if len(events) == 0 {
		return nil
	}
	event, err := transcript.UnmarshalAgentEvent(events[len(events)-1].Content)
	if err != nil {
		return fmt.Errorf("session: decode advertised command replacement set: %w", err)
	}
	if event.Type != acp.EventTypeAvailableCommands {
		return fmt.Errorf("session: advertised command event type changed to %q", event.Type)
	}
	session.replaceAdvertisedCommands(
		normalizeAdvertisedCommands(event.AvailableCommands.Values()),
		events[len(events)-1].Timestamp,
	)
	return nil
}

func normalizeAdvertisedCommands(commands []store.SessionAdvertisedCommand) []store.SessionAdvertisedCommand {
	result := make([]store.SessionAdvertisedCommand, 0, len(commands))
	seen := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		name := strings.TrimSpace(command.Name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		next := store.SessionAdvertisedCommand{
			Name:        name,
			Description: strings.TrimSpace(command.Description),
		}
		if command.Input != nil {
			next.Input = &store.SessionAdvertisedCommandInput{Hint: strings.TrimSpace(command.Input.Hint)}
		}
		result = append(result, next)
	}
	return result
}
