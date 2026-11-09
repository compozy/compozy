package demoseed

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const (
	northstarProjectFolderID   = "wsf_northstar_pay"
	northstarProjectFolderName = "Northstar Pay"
)

// sessionFolderStory groups a project's sessions the way its team would file them.
type sessionFolderStory struct {
	ID           string
	WorkspaceKey string
	Name         string
	SessionIDs   []string
}

func scenarioSessionFolders() []sessionFolderStory {
	return []sessionFolderStory{
		{
			ID: "sfd_northstar_settlement", WorkspaceKey: workspaceKeyPlatform, Name: "Settlement",
			SessionIDs: []string{
				sessionRetryBudgetID, sessionRetryReviewID, sessionSettlementRetryID, sessionPartnerReplayID,
				sessionExportSpecID, sessionExportRenameID,
			},
		},
		{
			ID: "sfd_northstar_checkout", WorkspaceKey: workspaceKeyPlatform, Name: "Checkout",
			SessionIDs: []string{sessionPixRefundsID, sessionRollbackDrillID},
		},
		{
			ID: "sfd_northstar_release_train", WorkspaceKey: workspaceKeyPlatform, Name: "Release train",
			SessionIDs: []string{sessionReleaseTrainID, sessionWebhookSecretsID, sessionDocsRefreshID},
		},
		{
			ID: "sfd_northstar_brazil_launch", WorkspaceKey: workspaceKeyLaunch, Name: "Brazil launch",
			SessionIDs: []string{sessionLaunchDecisionID, sessionComplianceCopyID, sessionSupportHandoffID},
		},
		{
			ID: "sfd_northstar_risk", WorkspaceKey: workspaceKeyLaunch, Name: "Risk and disputes",
			SessionIDs: []string{
				sessionChargebackBackfillID, sessionChargebackWatchID, sessionFraudSweepID, sessionDisputePlaybookID,
			},
		},
	}
}

// seedSessionFolders files sessions into folders and both projects under one project folder,
// through the same layout writes the session list's drag and folder menus use.
func seedSessionFolders(ctx context.Context, db *globaldb.GlobalDB, state *scenario) (int, error) {
	defer state.writeClock.reset()
	state.writeClock.at(state.clock.daysAgo(30))
	stories := scenarioSessionFolders()
	for _, story := range stories {
		record, err := state.recordFor(story.WorkspaceKey)
		if err != nil {
			return 0, err
		}
		scope := store.SessionLayoutScope{ProfileID: store.DefaultProfileID, WorkspaceID: record.ID}
		if _, err := db.CreateSessionFolder(ctx, scope, story.ID, story.Name); err != nil {
			return 0, fmt.Errorf("demo seed: create session folder %q: %w", story.Name, err)
		}
		for _, sessionID := range story.SessionIDs {
			if err := db.MoveSession(ctx, scope, sessionID, story.ID); err != nil {
				return 0, fmt.Errorf("demo seed: file session %q in %q: %w", sessionID, story.Name, err)
			}
		}
	}
	if _, err := db.CreateWorkspaceFolder(
		ctx, store.DefaultProfileID, northstarProjectFolderID, northstarProjectFolderName,
	); err != nil {
		return 0, fmt.Errorf("demo seed: create project folder: %w", err)
	}
	for _, workspace := range state.workspaces {
		record, err := state.recordFor(workspace.Key)
		if err != nil {
			return 0, err
		}
		if err := db.MoveWorkspace(ctx, store.DefaultProfileID, record.ID, northstarProjectFolderID); err != nil {
			return 0, fmt.Errorf("demo seed: file project %q: %w", record.Name, err)
		}
	}
	return len(stories), nil
}

// removeProjectFolder clears the profile-level folder; session folders leave with their workspace.
func removeProjectFolder(ctx context.Context, db *globaldb.GlobalDB) error {
	err := db.DeleteWorkspaceFolder(ctx, store.DefaultProfileID, northstarProjectFolderID)
	if err != nil && !errors.Is(err, store.ErrWorkspaceFolderNotFound) {
		return fmt.Errorf("demo seed: replace project folder: %w", err)
	}
	return nil
}
