package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonLoopAPIService) startSessionGoal(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	objective string,
	runtime *looppkg.RuntimeSpec,
	actor taskpkg.ActorContext,
) (session.GoalDispatchDecision, error) {
	contract, err := session.ParseGoalObjective(objective)
	if err != nil {
		return s.commandErrorDecision(ctx, workspaceID, sessionID, err)
	}
	prepared, decision, err := s.prepareSessionGoalDefinition(
		ctx,
		workspaceID,
		sessionID,
	)
	if err != nil || decision.Result != nil {
		return decision, err
	}
	runtime, err = s.resolveGoalRuntime(ctx, workspaceID, prepared.profileID, runtime, prepared.originProvider, prepared.originModel)
	if err != nil {
		return s.commandErrorDecision(ctx, workspaceID, sessionID, err)
	}
	definition := buildSessionGoalDefinition(
		contract,
		prepared.agentName,
		prepared.judgeModel,
		prepared.maxTurns,
	)
	run, err := s.aggregate.StartInline(
		ctx,
		looppkg.WorkspaceID(strings.TrimSpace(workspaceID)),
		definition,
		goalStartInputs(prepared.profileID, runtime),
		prepared.origin,
		actor,
	)
	if err != nil {
		return s.commandErrorDecision(ctx, workspaceID, sessionID, err)
	}
	snapshot, err := s.requireSessionGoalSnapshot(ctx, workspaceID, sessionID, run.ID)
	if err != nil {
		return session.GoalDispatchDecision{}, err
	}
	return goalCommandSuccessDecision(session.GoalOutcomeStarted, snapshot, nil), nil
}

func (s *daemonLoopAPIService) replaceSessionGoal(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	expectedRunID string,
	objective string,
	runtime *looppkg.RuntimeSpec,
	actor taskpkg.ActorContext,
) (session.GoalDispatchDecision, error) {
	contract, err := session.ParseGoalObjective(objective)
	if err != nil {
		return s.commandErrorDecision(ctx, workspaceID, sessionID, err)
	}
	prepared, decision, err := s.prepareSessionGoalDefinition(
		ctx,
		workspaceID,
		sessionID,
	)
	if err != nil || decision.Result != nil {
		return decision, err
	}
	runtime, err = s.resolveGoalRuntime(ctx, workspaceID, prepared.profileID, runtime, prepared.originProvider, prepared.originModel)
	if err != nil {
		return s.commandErrorDecision(ctx, workspaceID, sessionID, err)
	}
	definition := buildSessionGoalDefinition(
		contract,
		prepared.agentName,
		prepared.judgeModel,
		prepared.maxTurns,
	)
	replaced, err := s.aggregate.ReplaceInline(
		ctx,
		looppkg.RunID(strings.TrimSpace(expectedRunID)),
		looppkg.WorkspaceID(strings.TrimSpace(workspaceID)),
		definition,
		goalStartInputs(prepared.profileID, runtime),
		prepared.origin,
		actor,
	)
	if err != nil {
		return s.commandErrorDecision(ctx, workspaceID, sessionID, err)
	}
	if replaced.Run == nil {
		return session.GoalDispatchDecision{}, errors.New("daemon: Goal replacement returned no Run")
	}
	snapshot, err := s.requireSessionGoalSnapshot(ctx, workspaceID, sessionID, replaced.Run.ID)
	if err != nil {
		return session.GoalDispatchDecision{}, err
	}
	replacedRunID := string(replaced.ReplacedRunID)
	return goalCommandSuccessDecision(session.GoalOutcomeReplaced, snapshot, &replacedRunID), nil
}

type sessionGoalDefinitionInput struct {
	origin         looppkg.RunOrigin
	profileID      string
	originProvider string
	originModel    string
	agentName      string
	judgeModel     string
	maxTurns       int
}

type sessionGoalOriginDetails struct {
	origin      looppkg.RunOrigin
	profile     store.SessionCreationProfile
	activeModel string
}

func (s *daemonLoopAPIService) prepareSessionGoalDefinition(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (
	sessionGoalDefinitionInput,
	session.GoalDispatchDecision,
	error,
) {
	details, reason, err := s.sessionGoalOrigin(ctx, workspaceID, sessionID)
	if err != nil {
		return sessionGoalDefinitionInput{}, session.GoalDispatchDecision{}, err
	}
	if reason != "" {
		return sessionGoalDefinitionInput{}, goalCommandErrorDecision(reason, nil), nil
	}
	cfg, err := resolveLoopProfileConfig(
		ctx,
		s.homePaths,
		s.workspaceResolver,
		looppkg.WorkspaceID(strings.TrimSpace(workspaceID)),
		details.profile.ProfileID,
	)
	if err != nil {
		return sessionGoalDefinitionInput{}, session.GoalDispatchDecision{}, err
	}
	judgeModel := strings.TrimSpace(details.profile.Model)
	if judgeModel == "" {
		judgeModel = strings.TrimSpace(details.activeModel)
	}
	return sessionGoalDefinitionInput{
		origin:         details.origin,
		profileID:      details.profile.ProfileID,
		originProvider: details.profile.Provider,
		originModel:    strings.TrimSpace(firstNonEmptyGoalRuntime(details.profile.Model, details.activeModel)),
		agentName:      details.profile.AgentName,
		judgeModel:     judgeModel,
		maxTurns:       cfg.Goals.MaxTurns,
	}, session.GoalDispatchDecision{}, nil
}

func firstNonEmptyGoalRuntime(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (s *daemonLoopAPIService) resolveGoalRuntime(
	ctx context.Context,
	workspaceID string,
	profileID string,
	runtime *looppkg.RuntimeSpec,
	originProvider string,
	originModel string,
) (*looppkg.RuntimeSpec, error) {
	if runtime == nil {
		return nil, nil
	}
	resolved := *runtime
	resolved.ACPOptions = cloneLoopACPOptions(runtime.ACPOptions)
	provider := compozyconfig.CanonicalProviderName(resolved.Provider)
	if provider == "" {
		return nil, looppkg.NewRuntimeValidationError(looppkg.RuntimeValidationItem{
			Field: watchEventsPayloadProviderKey, Value: resolved.Provider, Reason: daemonRequiredValue,
		})
	}
	cfg, err := resolveLoopProfileConfig(
		ctx,
		s.homePaths,
		s.workspaceResolver,
		looppkg.WorkspaceID(strings.TrimSpace(workspaceID)),
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve Goal runtime config: %w", err)
	}
	providerConfig, err := cfg.ResolveProvider(provider)
	if err != nil {
		return nil, looppkg.NewRuntimeValidationError(looppkg.RuntimeValidationItem{
			Field: watchEventsPayloadProviderKey, Value: resolved.Provider, Reason: "unknown_provider",
		})
	}
	resolved.Provider = provider
	if strings.TrimSpace(resolved.Model) == "" {
		if provider == compozyconfig.CanonicalProviderName(originProvider) {
			resolved.Model = strings.TrimSpace(originModel)
		}
		if strings.TrimSpace(resolved.Model) == "" {
			resolved.Model = strings.TrimSpace(providerConfig.Models.Default)
		}
	}
	if strings.TrimSpace(resolved.Model) == "" {
		return nil, looppkg.NewRuntimeValidationError(looppkg.RuntimeValidationItem{
			Field: watchEventsPayloadModelKey, Value: "", Reason: "missing_runtime_model",
		})
	}
	return &resolved, nil
}

func (s *daemonLoopAPIService) sessionGoalOrigin(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (sessionGoalOriginDetails, session.GoalReasonCode, error) {
	if s.sessionStatus == nil {
		return sessionGoalOriginDetails{}, "", errors.New(
			"daemon: Goal origin session reader is unavailable",
		)
	}
	info, err := s.sessionStatus.Status(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		if errors.Is(err, store.ErrSessionNotFound) {
			return sessionGoalOriginDetails{},
				session.GoalReasonCode(looppkg.ReasonCodeGoalOriginInvalid), nil
		}
		return sessionGoalOriginDetails{}, "", fmt.Errorf(
			"daemon: read Goal origin session %q: %w",
			sessionID,
			err,
		)
	}
	if info == nil || info.State != session.StateActive {
		return sessionGoalOriginDetails{},
			session.GoalReasonCode(looppkg.ReasonCodeGoalOriginInvalid), nil
	}
	if strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(workspaceID) {
		return sessionGoalOriginDetails{},
			session.GoalReasonCode(looppkg.ReasonCodeGoalOriginWorkspaceMismatch), nil
	}
	if s.creationStore == nil {
		return s.unavailableSessionGoalOrigin(ctx, workspaceID, info.ID, "creation_store_missing")
	}
	identity, err := s.creationStore.GetSessionCreationIdentity(ctx, info.ID)
	if err != nil {
		if errors.Is(err, store.ErrSessionNotFound) {
			return s.unavailableSessionGoalOrigin(ctx, workspaceID, info.ID, "creation_identity_missing")
		}
		return sessionGoalOriginDetails{}, "", fmt.Errorf(
			"daemon: read Goal origin creation identity for session %q: %w",
			info.ID,
			err,
		)
	}
	if err := identity.Validate(); err != nil {
		return s.unavailableSessionGoalOrigin(ctx, workspaceID, info.ID, "creation_identity_invalid")
	}
	profile, err := s.creationStore.GetSessionCreationProfile(ctx, identity.CreationProfileRef)
	if err != nil {
		if errors.Is(err, store.ErrSessionNotFound) {
			return s.unavailableSessionGoalOrigin(ctx, workspaceID, info.ID, "creation_profile_missing")
		}
		return sessionGoalOriginDetails{}, "", fmt.Errorf(
			"daemon: read Goal origin creation profile %q: %w",
			identity.CreationProfileRef,
			err,
		)
	}
	if err := validateSessionGoalProfile(profile, identity, info, workspaceID); err != nil {
		return s.unavailableSessionGoalOrigin(ctx, workspaceID, info.ID, err.Error())
	}
	return sessionGoalOriginDetails{
		origin: looppkg.RunOrigin{
			Kind: looppkg.RunOriginSession, SessionID: info.ID,
			CreationProfileRef: identity.CreationProfileRef,
			PolicySpecDigest:   identity.PolicySpecDigest,
			CreationDigest:     identity.CreationDigest,
		},
		profile:     profile,
		activeModel: info.Model,
	}, "", nil
}

func (s *daemonLoopAPIService) unavailableSessionGoalOrigin(
	ctx context.Context, workspaceID, sessionID, check string,
) (sessionGoalOriginDetails, session.GoalReasonCode, error) {
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.WarnContext(ctx, "Goal origin profile unavailable",
		"workspace_id", workspaceID, "session_id", sessionID, "check", check)
	return sessionGoalOriginDetails{}, session.GoalReasonCode(looppkg.ReasonCodeGoalOriginProfileUnavailable), nil
}

func goalStartInputs(
	profileID string,
	runtime *looppkg.RuntimeSpec,

) looppkg.Inputs {
	inputs := looppkg.Inputs{
		ProfileID: profileID,
	}
	if runtime != nil {
		worker := *runtime
		worker.ACPOptions = cloneLoopACPOptions(runtime.ACPOptions)
		inputs.ConfigOverrides.RuntimeDefaults = &looppkg.RuntimeDefaults{Worker: worker}
	}
	return inputs
}

func validateSessionGoalProfile(
	profile store.SessionCreationProfile,
	identity store.SessionCreationIdentity,
	info *session.Info,
	workspaceID string,
) error {
	profile = store.NormalizeSessionCreationProfile(profile)
	if err := profile.Validate(); err != nil {
		return errors.New("creation_profile_invalid")
	}
	profileRef, err := profile.Ref()
	if err != nil {
		return errors.New("creation_profile_ref_invalid")
	}
	policyDigest, err := profile.PolicySpecDigest()
	if err != nil {
		return errors.New("creation_policy_digest_invalid")
	}
	switch {
	case profileRef != strings.TrimSpace(identity.CreationProfileRef):
		return errors.New("creation_profile_ref_mismatch")
	case policyDigest != strings.TrimSpace(identity.PolicySpecDigest):
		return errors.New("creation_policy_digest_mismatch")
	case profile.WorkspaceID != strings.TrimSpace(workspaceID):
		return errors.New("creation_workspace_mismatch")
	case info == nil:
		return errors.New("origin_session_missing")
	case profile.AgentName != strings.TrimSpace(info.AgentName):
		return errors.New("creation_agent_mismatch")
	}
	return nil
}

func (s *daemonLoopAPIService) requireSessionGoalSnapshot(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	wantRunID looppkg.RunID,
) (*session.GoalSnapshot, error) {
	snapshot, err := s.GetSessionGoal(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil || strings.TrimSpace(snapshot.RunID) != strings.TrimSpace(string(wantRunID)) {
		return nil, fmt.Errorf("%w: post-operation Goal snapshot is unavailable", looppkg.ErrTransitionConflict)
	}
	return snapshot, nil
}
