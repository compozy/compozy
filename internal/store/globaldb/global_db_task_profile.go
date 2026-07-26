package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

var _ taskpkg.ExecutionProfileStore = (*TaskRepo)(nil)

const (
	profileRoleWorker      = "worker"
	profileRoleReview      = "review"
	profileRoleParticipant = "participant"

	profilePreferenceAllowed   = "allowed"
	profilePreferencePreferred = "preferred"
	profilePreferenceRequired  = "required"
)

// GetExecutionProfile returns the persisted typed execution profile for one task.
func (g *TaskRepo) GetExecutionProfile(
	ctx context.Context,
	taskID string,
) (taskpkg.ExecutionProfile, error) {
	if err := g.checkReady(ctx, "get task execution profile"); err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	trimmedID, err := requireTaskValue(taskID, "task execution profile task id")
	if err != nil {
		return taskpkg.ExecutionProfile{}, err
	}

	profile, found, err := loadExecutionProfile(ctx, g.db, trimmedID)
	if err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	if !found {
		return taskpkg.ExecutionProfile{}, taskpkg.ErrExecutionProfileNotFound
	}
	return profile, nil
}

// UpsertExecutionProfile replaces one task-owned execution profile and its selectors atomically.
func (g *TaskRepo) UpsertExecutionProfile(
	ctx context.Context,
	profile *taskpkg.ExecutionProfile,
) (stored taskpkg.ExecutionProfile, err error) {
	if err := g.checkReady(ctx, "upsert task execution profile"); err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	normalized, err := profile.Normalize(taskpkg.DefaultExecutionProfileValidationOptions())
	if err != nil {
		return taskpkg.ExecutionProfile{}, err
	}

	if err := g.withTaskImmediateTransaction(ctx, "upsert task execution profile", func(exec taskSQLExecutor) error {
		var storeErr error
		stored, storeErr = g.upsertExecutionProfileWithExecutor(ctx, exec, &normalized)
		return storeErr
	}); err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	return stored, nil
}

func (g *TaskRepo) upsertExecutionProfileWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) (taskpkg.ExecutionProfile, error) {
	normalized, err := profile.Normalize(taskpkg.DefaultExecutionProfileValidationOptions())
	if err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	if err := g.ensureTaskExistsWithExecutor(ctx, exec, normalized.TaskID); err != nil {
		return taskpkg.ExecutionProfile{}, err
	}

	now := g.now().UTC()
	existing, found, err := loadExecutionProfile(ctx, exec, normalized.TaskID)
	if err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	if found {
		normalized.CreatedAt = existing.CreatedAt
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = now
	}
	normalized.UpdatedAt = now

	if err := upsertExecutionProfileRow(ctx, exec, &normalized); err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	if err := replaceExecutionProfileSelectors(ctx, exec, &normalized); err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	return reloadExecutionProfile(ctx, exec, normalized.TaskID)
}

// DeleteExecutionProfile removes one profile and its selector rows.
func (g *TaskRepo) DeleteExecutionProfile(ctx context.Context, taskID string) error {
	if err := g.checkReady(ctx, "delete task execution profile"); err != nil {
		return err
	}
	trimmedID, err := requireTaskValue(taskID, "task execution profile task id")
	if err != nil {
		return err
	}

	return g.withTaskImmediateTransaction(ctx, "delete task execution profile", func(exec taskSQLExecutor) error {
		return deleteExecutionProfileWithExecutor(ctx, exec, trimmedID)
	})
}

func deleteExecutionProfileWithExecutor(ctx context.Context, exec taskSQLExecutor, taskID string) error {
	if err := deleteExecutionProfileSelectors(ctx, exec, taskID); err != nil {
		return err
	}
	affected, err := sqlcgen.New(exec).DeleteTaskExecutionProfile(ctx, taskID)
	if err != nil {
		return fmt.Errorf("store: delete task execution profile %q: %w", taskID, err)
	}
	if affected == 0 {
		return fmt.Errorf(
			"store: task execution profile %q: %w",
			taskID,
			taskpkg.ErrExecutionProfileNotFound,
		)
	}
	return nil
}

func loadExecutionProfile(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
) (taskpkg.ExecutionProfile, bool, error) {
	row, err := sqlcgen.New(exec).GetTaskExecutionProfile(ctx, taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.ExecutionProfile{}, false, nil
		}
		return taskpkg.ExecutionProfile{}, false, err
	}
	profile, err := executionProfileFromGenerated(row)
	if err != nil {
		return taskpkg.ExecutionProfile{}, false, err
	}
	if err := loadExecutionProfileSelectors(ctx, exec, &profile); err != nil {
		return taskpkg.ExecutionProfile{}, false, err
	}
	return profile, true, nil
}

func reloadExecutionProfile(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
) (taskpkg.ExecutionProfile, error) {
	profile, found, err := loadExecutionProfile(ctx, exec, taskID)
	if err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	if !found {
		return taskpkg.ExecutionProfile{}, taskpkg.ErrExecutionProfileNotFound
	}
	return profile, nil
}

func upsertExecutionProfileRow(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) error {
	networkMode, networkStrategy, networkChannel, networkBounds, err :=
		executionProfileParticipationColumns(profile.NetworkParticipation)
	if err != nil {
		return fmt.Errorf("store: normalize task execution profile network participation: %w", err)
	}
	if err := sqlcgen.New(exec).UpsertTaskExecutionProfile(ctx, sqlcgen.UpsertTaskExecutionProfileParams{
		TaskID:                 profile.TaskID,
		CoordinatorMode:        string(profile.Coordinator.Mode),
		CoordinatorAgentName:   profile.Coordinator.AgentName,
		CoordinatorProvider:    profile.Coordinator.Provider,
		CoordinatorModel:       profile.Coordinator.Model,
		CoordinatorGuidance:    profile.Coordinator.Guidance,
		WorkerMode:             string(profile.Worker.Mode),
		WorkerAgentName:        profile.Worker.AgentName,
		WorkerProvider:         profile.Worker.Provider,
		WorkerModel:            profile.Worker.Model,
		ReviewAgentName:        profile.Review.AgentName,
		ReviewProvider:         profile.Review.Provider,
		ReviewModel:            profile.Review.Model,
		SandboxMode:            string(profile.Sandbox.Mode),
		SandboxRef:             profile.Sandbox.SandboxRef,
		RuntimeMode:            string(profile.Runtime.Mode),
		CreatedAt:              store.FormatTimestamp(profile.CreatedAt),
		UpdatedAt:              store.FormatTimestamp(profile.UpdatedAt),
		NetworkMode:            networkMode,
		NetworkChannelStrategy: networkStrategy,
		NetworkChannel:         networkChannel,
		NetworkBoundsJson:      networkBounds,
	}); err != nil {
		return fmt.Errorf("store: upsert task execution profile %q: %w", profile.TaskID, err)
	}
	return nil
}

func replaceExecutionProfileSelectors(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) error {
	if err := deleteExecutionProfileSelectors(ctx, exec, profile.TaskID); err != nil {
		return err
	}
	if err := insertTaskProfileAgentSelectors(ctx, exec, profile); err != nil {
		return err
	}
	if err := insertTaskProfileChannelSelectors(ctx, exec, profile); err != nil {
		return err
	}
	if err := insertTaskProfilePeerSelectors(ctx, exec, profile); err != nil {
		return err
	}
	return insertTaskProfileCapabilitySelectors(ctx, exec, profile)
}

func deleteExecutionProfileSelectors(ctx context.Context, exec taskSQLExecutor, taskID string) error {
	queries := sqlcgen.New(exec)
	deletes := []struct {
		label string
		apply func(context.Context, string) error
	}{
		{label: "task_profile_agents", apply: queries.DeleteTaskProfileAgents},
		{label: "task_profile_channels", apply: queries.DeleteTaskProfileChannels},
		{label: "task_profile_peers", apply: queries.DeleteTaskProfilePeers},
		{label: "task_profile_capabilities", apply: queries.DeleteTaskProfileCapabilities},
	}
	for _, deletion := range deletes {
		if err := deletion.apply(ctx, taskID); err != nil {
			return fmt.Errorf("store: delete %s rows for task %q: %w", deletion.label, taskID, err)
		}
	}
	return nil
}

func insertTaskProfileAgentSelectors(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) error {
	rows := []profileSelectorRow{
		{profileRoleWorker, profilePreferenceAllowed, profile.Worker.AllowedAgentNames},
		{profileRoleWorker, profilePreferencePreferred, profile.Worker.PreferredAgentNames},
		{profileRoleReview, profilePreferenceAllowed, profile.Review.AllowedAgentNames},
		{profileRoleReview, profilePreferencePreferred, profile.Review.PreferredAgentNames},
		{profileRoleParticipant, profilePreferenceAllowed, profile.Participants.AllowedAgentNames},
		{profileRoleParticipant, profilePreferencePreferred, profile.Participants.PreferredAgentNames},
	}
	queries := sqlcgen.New(exec)
	for _, row := range rows {
		for _, value := range row.values {
			if err := queries.InsertTaskProfileAgent(ctx, sqlcgen.InsertTaskProfileAgentParams{
				TaskID: profile.TaskID, Role: row.role, Preference: row.preference, Value: value,
			}); err != nil {
				return fmt.Errorf("store: insert task_profile_agents selector for task %q: %w", profile.TaskID, err)
			}
		}
	}
	return nil
}

func insertTaskProfileChannelSelectors(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) error {
	rows := []profileSelectorRow{
		{profileRoleReview, profilePreferenceAllowed, profile.Review.AllowedChannelIDs},
		{profileRoleReview, profilePreferencePreferred, profile.Review.PreferredChannelIDs},
		{profileRoleParticipant, profilePreferenceAllowed, profile.Participants.AllowedChannelIDs},
		{profileRoleParticipant, profilePreferencePreferred, profile.Participants.PreferredChannelIDs},
	}
	queries := sqlcgen.New(exec)
	for _, row := range rows {
		for _, value := range row.values {
			if err := queries.InsertTaskProfileChannel(ctx, sqlcgen.InsertTaskProfileChannelParams{
				TaskID: profile.TaskID, Role: row.role, Preference: row.preference, Value: value,
			}); err != nil {
				return fmt.Errorf("store: insert task_profile_channels selector for task %q: %w", profile.TaskID, err)
			}
		}
	}
	return nil
}

func insertTaskProfilePeerSelectors(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) error {
	rows := []profileSelectorRow{
		{profileRoleReview, profilePreferenceAllowed, profile.Review.AllowedPeerIDs},
		{profileRoleReview, profilePreferencePreferred, profile.Review.PreferredPeerIDs},
		{profileRoleParticipant, profilePreferenceAllowed, profile.Participants.AllowedPeerIDs},
		{profileRoleParticipant, profilePreferencePreferred, profile.Participants.PreferredPeerIDs},
	}
	queries := sqlcgen.New(exec)
	for _, row := range rows {
		for _, value := range row.values {
			if err := queries.InsertTaskProfilePeer(ctx, sqlcgen.InsertTaskProfilePeerParams{
				TaskID: profile.TaskID, Role: row.role, Preference: row.preference, Value: value,
			}); err != nil {
				return fmt.Errorf("store: insert task_profile_peers selector for task %q: %w", profile.TaskID, err)
			}
		}
	}
	return nil
}

func insertTaskProfileCapabilitySelectors(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) error {
	rows := []profileSelectorRow{
		{profileRoleWorker, profilePreferenceRequired, profile.Worker.RequiredCapabilities},
		{profileRoleWorker, profilePreferencePreferred, profile.Worker.PreferredCapabilities},
		{profileRoleReview, profilePreferenceRequired, profile.Review.RequiredCapabilities},
		{profileRoleReview, profilePreferencePreferred, profile.Review.PreferredCapabilities},
		{profileRoleParticipant, profilePreferenceRequired, profile.Participants.RequiredCapabilities},
		{profileRoleParticipant, profilePreferencePreferred, profile.Participants.PreferredCapabilities},
	}
	queries := sqlcgen.New(exec)
	for _, row := range rows {
		for _, value := range row.values {
			if err := queries.InsertTaskProfileCapability(ctx, sqlcgen.InsertTaskProfileCapabilityParams{
				TaskID: profile.TaskID, Role: row.role, Preference: row.preference, Value: value,
			}); err != nil {
				return fmt.Errorf(
					"store: insert task_profile_capabilities selector for task %q: %w",
					profile.TaskID,
					err,
				)
			}
		}
	}
	return nil
}

type profileSelectorRow struct {
	role       string
	preference string
	values     []string
}

func loadExecutionProfileSelectors(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) error {
	if err := loadTaskProfileAgentSelectors(ctx, exec, profile); err != nil {
		return err
	}
	if err := loadTaskProfileChannelSelectors(ctx, exec, profile); err != nil {
		return err
	}
	if err := loadTaskProfilePeerSelectors(ctx, exec, profile); err != nil {
		return err
	}
	return loadTaskProfileCapabilitySelectors(ctx, exec, profile)
}

func loadTaskProfileAgentSelectors(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) error {
	rows, err := sqlcgen.New(exec).ListTaskProfileAgents(ctx, profile.TaskID)
	if err != nil {
		return fmt.Errorf("store: query task_profile_agents selectors for task %q: %w", profile.TaskID, err)
	}
	for _, generated := range rows {
		row := loadedProfileSelectorRow{generated.Role, generated.Preference, generated.Value}
		switch {
		case row.matches(profileRoleWorker, profilePreferenceAllowed):
			profile.Worker.AllowedAgentNames = append(profile.Worker.AllowedAgentNames, row.value)
		case row.matches(profileRoleWorker, profilePreferencePreferred):
			profile.Worker.PreferredAgentNames = append(profile.Worker.PreferredAgentNames, row.value)
		case row.matches(profileRoleReview, profilePreferenceAllowed):
			profile.Review.AllowedAgentNames = append(profile.Review.AllowedAgentNames, row.value)
		case row.matches(profileRoleReview, profilePreferencePreferred):
			profile.Review.PreferredAgentNames = append(profile.Review.PreferredAgentNames, row.value)
		case row.matches(profileRoleParticipant, profilePreferenceAllowed):
			profile.Participants.AllowedAgentNames = append(profile.Participants.AllowedAgentNames, row.value)
		case row.matches(profileRoleParticipant, profilePreferencePreferred):
			profile.Participants.PreferredAgentNames = append(profile.Participants.PreferredAgentNames, row.value)
		}
	}
	return nil
}

func loadTaskProfileChannelSelectors(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) error {
	rows, err := sqlcgen.New(exec).ListTaskProfileChannels(ctx, profile.TaskID)
	if err != nil {
		return fmt.Errorf("store: query task_profile_channels selectors for task %q: %w", profile.TaskID, err)
	}
	for _, generated := range rows {
		row := loadedProfileSelectorRow{generated.Role, generated.Preference, generated.Value}
		switch {
		case row.matches(profileRoleReview, profilePreferenceAllowed):
			profile.Review.AllowedChannelIDs = append(profile.Review.AllowedChannelIDs, row.value)
		case row.matches(profileRoleReview, profilePreferencePreferred):
			profile.Review.PreferredChannelIDs = append(profile.Review.PreferredChannelIDs, row.value)
		case row.matches(profileRoleParticipant, profilePreferenceAllowed):
			profile.Participants.AllowedChannelIDs = append(profile.Participants.AllowedChannelIDs, row.value)
		case row.matches(profileRoleParticipant, profilePreferencePreferred):
			profile.Participants.PreferredChannelIDs = append(profile.Participants.PreferredChannelIDs, row.value)
		}
	}
	return nil
}

func loadTaskProfilePeerSelectors(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) error {
	rows, err := sqlcgen.New(exec).ListTaskProfilePeers(ctx, profile.TaskID)
	if err != nil {
		return fmt.Errorf("store: query task_profile_peers selectors for task %q: %w", profile.TaskID, err)
	}
	for _, generated := range rows {
		row := loadedProfileSelectorRow{generated.Role, generated.Preference, generated.Value}
		switch {
		case row.matches(profileRoleReview, profilePreferenceAllowed):
			profile.Review.AllowedPeerIDs = append(profile.Review.AllowedPeerIDs, row.value)
		case row.matches(profileRoleReview, profilePreferencePreferred):
			profile.Review.PreferredPeerIDs = append(profile.Review.PreferredPeerIDs, row.value)
		case row.matches(profileRoleParticipant, profilePreferenceAllowed):
			profile.Participants.AllowedPeerIDs = append(profile.Participants.AllowedPeerIDs, row.value)
		case row.matches(profileRoleParticipant, profilePreferencePreferred):
			profile.Participants.PreferredPeerIDs = append(profile.Participants.PreferredPeerIDs, row.value)
		}
	}
	return nil
}

func loadTaskProfileCapabilitySelectors(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) error {
	rows, err := sqlcgen.New(exec).ListTaskProfileCapabilities(ctx, profile.TaskID)
	if err != nil {
		return fmt.Errorf("store: query task_profile_capabilities selectors for task %q: %w", profile.TaskID, err)
	}
	for _, generated := range rows {
		row := loadedProfileSelectorRow{generated.Role, generated.Preference, generated.Value}
		switch {
		case row.matches(profileRoleWorker, profilePreferenceRequired):
			profile.Worker.RequiredCapabilities = append(profile.Worker.RequiredCapabilities, row.value)
		case row.matches(profileRoleWorker, profilePreferencePreferred):
			profile.Worker.PreferredCapabilities = append(profile.Worker.PreferredCapabilities, row.value)
		case row.matches(profileRoleReview, profilePreferenceRequired):
			profile.Review.RequiredCapabilities = append(profile.Review.RequiredCapabilities, row.value)
		case row.matches(profileRoleReview, profilePreferencePreferred):
			profile.Review.PreferredCapabilities = append(profile.Review.PreferredCapabilities, row.value)
		case row.matches(profileRoleParticipant, profilePreferenceRequired):
			profile.Participants.RequiredCapabilities = append(profile.Participants.RequiredCapabilities, row.value)
		case row.matches(profileRoleParticipant, profilePreferencePreferred):
			profile.Participants.PreferredCapabilities = append(profile.Participants.PreferredCapabilities, row.value)
		}
	}
	return nil
}

type loadedProfileSelectorRow struct {
	role       string
	preference string
	value      string
}

func (r loadedProfileSelectorRow) matches(role string, preference string) bool {
	return r.role == role && r.preference == preference
}
