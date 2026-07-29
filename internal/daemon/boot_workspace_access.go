package daemon

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (d *Daemon) bootWorkspaceAccess(state *bootState, sessions SessionManager) error {
	if state == nil {
		return errors.New("daemon: boot state is required for workspace access")
	}
	modeSource, err := newWorkspaceAccessModeSource(sessions)
	if err != nil {
		return err
	}
	consent := newWorkspaceAccessConsentCache()
	audit, err := newWorkspaceAccessAuditEmitter(store.EventSummaryStore(state.registry), d.now)
	if err != nil {
		return err
	}
	policy, err := workspaceaccess.New(workspaceaccess.Deps{
		Modes:   modeSource,
		Consent: consent,
		Audit:   audit,
		Log:     state.logger,
	})
	if err != nil {
		return fmt.Errorf("daemon: create workspace access policy: %w", err)
	}
	state.accessPolicy = policy
	state.accessConsent = consent
	return nil
}
