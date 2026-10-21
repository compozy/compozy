package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/terminal"
)

type profileUserJournal struct{ state *bootState }

var _ profile.UserJournalRemover = profileUserJournal{}

func (p profileUserJournal) RemoveUserJournal(ctx context.Context, profileID string) error {
	if p.state == nil || p.state.terminals == nil {
		return errors.New("daemon: terminal service is unavailable for profile deletion")
	}
	if err := p.state.terminals.ArchiveProfile(ctx, profileID); err != nil {
		return err
	}
	return p.state.terminals.Journal().RemoveWorkspace(ctx, terminal.JournalWorkspaceID("", profileID))
}
