package journal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/terminal/idgen"
)

const commandIDAllocationAttempts = 2

// ReserveCommandID returns an identity that is absent from durable and in-flight commands.
func (s *Service) ReserveCommandID(ctx context.Context, workspaceID string) (string, error) {
	return s.allocateCommandID(ctx, workspaceID, true)
}

func (s *Service) reserveObservedCommandID(ctx context.Context, workspaceID string) (string, error) {
	return s.allocateCommandID(ctx, workspaceID, false)
}

func (s *Service) allocateCommandID(
	ctx context.Context,
	workspaceID string,
	requireDurableCheck bool,
) (string, error) {
	if s == nil || strings.TrimSpace(workspaceID) == "" {
		return "", errors.New("terminal journal: workspace id is required for command identity")
	}
	db, err := s.databases.Open(ctx, workspaceID)
	if err != nil && requireDurableCheck {
		return "", fmt.Errorf("terminal journal: open workspace store for command identity: %w", err)
	}
	for range commandIDAllocationAttempts {
		id, generateErr := idgen.New(s.entropy, "cmd-")
		if generateErr != nil {
			return "", fmt.Errorf("terminal journal: generate command id: %w", generateErr)
		}
		if !s.claimCommandID(workspaceID, id) {
			continue
		}
		if err != nil {
			s.logger.Warn("terminal journal: durable command identity check deferred",
				"workspace_id", workspaceID, "command_id", id, "error", err,
			)
			return id, nil
		}
		exists, existsErr := db.TerminalCommandIDExists(ctx, id)
		if existsErr != nil && requireDurableCheck {
			s.ReleaseCommandID(workspaceID, id)
			return "", fmt.Errorf("terminal journal: check command id %q: %w", id, existsErr)
		}
		if existsErr != nil {
			s.logger.Warn("terminal journal: durable command identity check deferred",
				"workspace_id", workspaceID, "command_id", id, "error", existsErr,
			)
			return id, nil
		}
		if exists {
			s.ReleaseCommandID(workspaceID, id)
			continue
		}
		return id, nil
	}
	return "", fmt.Errorf(
		"terminal journal: allocate command id after %d collisions",
		commandIDAllocationAttempts,
	)
}

// ReleaseCommandID releases an identity after persistence or abandoned admission.
func (s *Service) ReleaseCommandID(workspaceID, commandID string) {
	if s == nil {
		return
	}
	s.commandIDMu.Lock()
	delete(s.reservedCommandIDs, commandIDReservationKey(workspaceID, commandID))
	s.commandIDMu.Unlock()
}

func (s *Service) claimCommandID(workspaceID, commandID string) bool {
	key := commandIDReservationKey(workspaceID, commandID)
	s.commandIDMu.Lock()
	defer s.commandIDMu.Unlock()
	if _, exists := s.reservedCommandIDs[key]; exists {
		return false
	}
	s.reservedCommandIDs[key] = struct{}{}
	return true
}

func commandIDReservationKey(workspaceID, commandID string) string {
	return workspaceID + "\x00" + commandID
}

// ReserveRecordingID returns an identity absent from durable and in-flight recordings.
func (s *Service) ReserveRecordingID(ctx context.Context, workspaceID string) (string, error) {
	if s == nil || strings.TrimSpace(workspaceID) == "" {
		return "", errors.New("terminal journal: workspace id is required for recording identity")
	}
	db, err := s.databases.Open(ctx, workspaceID)
	if err != nil {
		return "", fmt.Errorf("terminal journal: open workspace store for recording identity: %w", err)
	}
	for range commandIDAllocationAttempts {
		id, generateErr := idgen.New(s.entropy, "rec-")
		if generateErr != nil {
			return "", fmt.Errorf("terminal journal: generate recording id: %w", generateErr)
		}
		if !s.reserveRecordingID(workspaceID, id) {
			continue
		}
		exists, existsErr := db.TerminalRecordingIDExists(ctx, id)
		if existsErr != nil {
			s.ReleaseRecordingID(workspaceID, id)
			return "", fmt.Errorf("terminal journal: check recording id %q: %w", id, existsErr)
		}
		if exists {
			s.ReleaseRecordingID(workspaceID, id)
			continue
		}
		return id, nil
	}
	return "", fmt.Errorf(
		"terminal journal: allocate recording id after %d collisions",
		commandIDAllocationAttempts,
	)
}

// ReleaseRecordingID releases an identity after persistence or abandoned admission.
func (s *Service) ReleaseRecordingID(workspaceID, recordingID string) {
	if s == nil {
		return
	}
	s.recordingIDMu.Lock()
	delete(s.reservedRecordingIDs, commandIDReservationKey(workspaceID, recordingID))
	s.recordingIDMu.Unlock()
}

func (s *Service) reserveRecordingID(workspaceID, recordingID string) bool {
	key := commandIDReservationKey(workspaceID, recordingID)
	s.recordingIDMu.Lock()
	defer s.recordingIDMu.Unlock()
	if _, exists := s.reservedRecordingIDs[key]; exists {
		return false
	}
	s.reservedRecordingIDs[key] = struct{}{}
	return true
}

func (s *Service) newArtifactID() (string, error) {
	id, err := idgen.New(s.entropy, "art-")
	if err != nil {
		return "", fmt.Errorf("terminal journal: generate artifact id: %w", err)
	}
	return id, nil
}
