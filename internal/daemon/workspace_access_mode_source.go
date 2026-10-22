package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type workspaceAccessSessionStatus interface {
	Status(context.Context, string) (*session.Info, error)
}

type workspaceAccessModeSource struct {
	sessions workspaceAccessSessionStatus
}

var _ workspaceaccess.SessionFactsSource = (*workspaceAccessModeSource)(nil)

func newWorkspaceAccessModeSource(sessions workspaceAccessSessionStatus) (*workspaceAccessModeSource, error) {
	if sessions == nil {
		return nil, errors.New("daemon: workspace access session status source is required")
	}
	return &workspaceAccessModeSource{sessions: sessions}, nil
}

func (s *workspaceAccessModeSource) SessionFacts(
	ctx context.Context,
	sessionID string,
) (workspaceaccess.SessionFacts, error) {
	if ctx == nil {
		return workspaceaccess.SessionFacts{}, errors.New("daemon: workspace access mode context is required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return workspaceaccess.SessionFacts{}, errors.New("daemon: workspace access session id is required")
	}
	info, err := s.sessions.Status(ctx, sessionID)
	if err != nil {
		return workspaceaccess.SessionFacts{}, fmt.Errorf(
			"daemon: read workspace access session %q: %w",
			sessionID,
			err,
		)
	}
	if info == nil {
		return workspaceaccess.SessionFacts{}, fmt.Errorf(
			"daemon: workspace access session %q returned no status",
			sessionID,
		)
	}
	if info.State == session.StateStopped {
		return workspaceaccess.SessionFacts{}, fmt.Errorf("daemon: workspace access session %q is stopped", sessionID)
	}
	mode := workspaceaccess.Mode(strings.TrimSpace(info.EffectivePermissions))
	if mode == "" {
		return workspaceaccess.SessionFacts{}, fmt.Errorf(
			"daemon: workspace access session %q has no effective permission mode",
			sessionID,
		)
	}
	return workspaceaccess.SessionFacts{
		Mode:            mode,
		WorkspaceAccess: info.EffectiveWorkspaceAccess,
		ProfileID:       info.ProfileID,
		WorkspaceID:     info.WorkspaceID,
	}, nil
}
