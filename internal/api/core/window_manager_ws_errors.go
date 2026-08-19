package core

import (
	"errors"

	"github.com/compozy/compozy/internal/windowmanager"
)

func (s *windowManagerSocket) subscriptionError() error {
	runErr := s.subscription.Err()
	if runErr == nil {
		return nil
	}
	return errors.Join(runErr, s.writeError(runErr))
}

func (s *windowManagerSocket) writeSubscriptionUpdate(update windowmanager.SubscriptionUpdate) error {
	encoded, err := encodeWindowManagerUpdate(s.workspaceID, update)
	if err != nil {
		return errors.Join(err, s.writeError(err))
	}
	return encoded.write(s)
}

func (s *windowManagerSocket) clientCommandError() error {
	err := s.clientCommands.Err()
	if err == nil {
		err = windowmanager.ErrClientDisconnected
	}
	if errors.Is(err, windowmanager.ErrClientNotFound) ||
		errors.Is(err, windowmanager.ErrWorkspaceNotFound) ||
		errors.Is(err, windowmanager.ErrClosed) {
		return errors.Join(err, s.writeError(err))
	}
	return err
}

func (s *windowManagerSocket) decorateReadError(err error) error {
	if errors.Is(err, errWindowManagerClientMessage) {
		return errors.Join(err, s.writeError(windowmanager.ErrInvalidCommand))
	}
	if errors.Is(err, windowmanager.ErrClientNotFound) ||
		errors.Is(err, windowmanager.ErrWorkspaceNotFound) ||
		errors.Is(err, windowmanager.ErrClosed) {
		return errors.Join(err, s.writeError(err))
	}
	return err
}
