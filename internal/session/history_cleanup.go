package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/sessioncleanup"
	"github.com/compozy/compozy/internal/store"
)

// PreviewHistoryCleanup plans an archived-history cleanup without deleting anything.
func (m *Manager) PreviewHistoryCleanup(
	ctx context.Context,
	request sessioncleanup.Request,
) (sessioncleanup.Plan, error) {
	service, err := m.historyCleanupService()
	if err != nil {
		return sessioncleanup.Plan{}, err
	}
	return service.Preview(ctx, request)
}

// StartHistoryCleanup plans an archived-history cleanup and deletes it in the
// background, a few archived sessions at a time, each through DeleteArchived.
func (m *Manager) StartHistoryCleanup(
	ctx context.Context,
	request sessioncleanup.Request,
) (sessioncleanup.Run, sessioncleanup.Plan, error) {
	service, err := m.historyCleanupService()
	if err != nil {
		return sessioncleanup.Run{}, sessioncleanup.Plan{}, err
	}
	return service.Start(ctx, request)
}

// HistoryCleanupRun returns the profile's running or most recent cleanup run.
func (m *Manager) HistoryCleanupRun(profileID string) (sessioncleanup.Run, bool, error) {
	service, err := m.historyCleanupService()
	if err != nil {
		return sessioncleanup.Run{}, false, err
	}
	run, ok := service.Current(profileID)
	return run, ok, nil
}

// StopHistoryCleanup stops the profile's running cleanup after its deletes in
// flight; a non-empty runID stops only that run.
func (m *Manager) StopHistoryCleanup(ctx context.Context, profileID, runID string) (sessioncleanup.Run, error) {
	service, err := m.historyCleanupService()
	if err != nil {
		return sessioncleanup.Run{}, err
	}
	return service.Stop(ctx, profileID, runID)
}

// historyCleanupService builds the cleanup service on first use; the manager
// owns its runs and stops them at shutdown.
func (m *Manager) historyCleanupService() (*sessioncleanup.Service, error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	m.historyCleanupMu.Lock()
	defer m.historyCleanupMu.Unlock()
	if m.historyCleanup != nil {
		return m.historyCleanup, nil
	}
	// A manager shutting down builds no service: nothing would join its runs.
	if m.historyCleanupClosed {
		return nil, sessioncleanup.ErrShuttingDown
	}
	lister, ok := m.sessionCatalog.(store.SessionHistoryCleanupStore)
	if !ok || lister == nil {
		return nil, ErrSessionArchiveUnavailable
	}
	service, err := sessioncleanup.NewService(
		lister,
		historyCleanupDeleter{manager: m},
		sessioncleanup.WithPublisher(m.PublishHistoryCleanupChanged),
		sessioncleanup.WithLogger(m.logger),
	)
	if err != nil {
		return nil, fmt.Errorf("session: build history cleanup: %w", err)
	}
	m.historyCleanup = service
	return service, nil
}

func (m *Manager) shutdownHistoryCleanup(ctx context.Context) error {
	m.historyCleanupMu.Lock()
	m.historyCleanupClosed = true
	service := m.historyCleanup
	m.historyCleanupMu.Unlock()
	if service == nil {
		return nil
	}
	return service.Shutdown(ctx)
}

// historyCleanupDeleter reports a session already gone the way the cleanup
// service expects, whichever layer noticed it.
type historyCleanupDeleter struct {
	manager *Manager
}

func (d historyCleanupDeleter) DeleteArchived(ctx context.Context, target store.ArchivedSessionDelete) error {
	err := d.manager.DeleteArchived(ctx, target)
	if errors.Is(err, ErrSessionNotFound) || errors.Is(err, store.ErrSessionNotFound) {
		_, readErr := d.manager.durableSessionInfoForLifecycleTransition(ctx, target.SessionID)
		if errors.Is(readErr, ErrSessionNotFound) {
			return nil
		}
		return errors.Join(err, readErr)
	}
	return err
}
