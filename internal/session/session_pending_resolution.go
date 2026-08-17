package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session/inputqueue"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

var errPendingInteractionNotOrphaned = errors.New("session: pending interaction turn is still live")

// ApprovalResult reports the canonical outcome of one permission decision.
type ApprovalResult struct {
	Outcome          string
	InteractionID    string
	RequestID        string
	Decision         string
	ResolvedDecision string
}

// PendingInteractionResolution reports one canonical orphan-resolution attempt.
type PendingInteractionResolution struct {
	Outcome       string
	InteractionID string
	RequestID     string
	Resolution    string
}

// ApprovePermission resolves either a live provider request or its restart-orphaned record.
func (m *Manager) ApprovePermission(
	ctx context.Context,
	id string,
	req acp.ApproveRequest,
) (ApprovalResult, error) {
	if ctx == nil {
		return ApprovalResult{}, errors.New("session: approval context is required")
	}
	if err := req.Validate(); err != nil {
		return ApprovalResult{}, err
	}
	target := strings.TrimSpace(id)
	if target == "" {
		return ApprovalResult{}, errors.New("session: session id is required")
	}
	identity := strings.TrimSpace(req.RequestID)
	if identity == "" {
		identity = strings.TrimSpace(req.TurnID)
	}
	if m.attentionStore != nil {
		interaction, err := m.interactionForResolution(ctx, target, store.PendingInteractionKindPermission, identity)
		if err == nil && interaction.Status != store.PendingInteractionStatusPending {
			return m.resolveOrphanedPermission(ctx, target, interaction, req.Decision)
		}
		if err != nil && !errors.Is(err, store.ErrPendingInteractionNotFound) {
			return ApprovalResult{}, err
		}
	}

	active, ok := m.Get(target)
	if !ok {
		meta, err := m.readMetaWithContext(ctx, target)
		if err != nil {
			return ApprovalResult{}, err
		}
		return ApprovalResult{}, fmt.Errorf("%w: %s (%s)", ErrSessionNotActive, target, meta.State)
	}
	if err := active.ApprovePermission(ctx, req); err != nil {
		mappedErr := approvalError(target, err)
		if m.attentionStore == nil ||
			(!errors.Is(mappedErr, ErrPendingPermissionNotFound) &&
				!errors.Is(mappedErr, ErrPendingPermissionConflict)) {
			return ApprovalResult{}, mappedErr
		}
		winner, lookupErr := m.interactionForResolution(
			ctx,
			target,
			store.PendingInteractionKindPermission,
			identity,
		)
		if lookupErr != nil {
			if errors.Is(lookupErr, store.ErrPendingInteractionNotFound) {
				return ApprovalResult{}, mappedErr
			}
			return ApprovalResult{}, lookupErr
		}
		if winner.Status == store.PendingInteractionStatusPending {
			return ApprovalResult{}, mappedErr
		}
		return m.resolveOrphanedPermission(ctx, target, winner, req.Decision)
	}
	return ApprovalResult{
		Outcome:   store.PendingInteractionOutcomeApplied,
		RequestID: identity,
		Decision:  strings.TrimSpace(req.Decision),
	}, nil
}

func approvalError(sessionID string, err error) error {
	switch {
	case errors.Is(err, ErrSessionNotActive):
		return err
	case errors.Is(err, acp.ErrPendingPermissionNotFound):
		return fmt.Errorf("%w: %s", ErrPendingPermissionNotFound, sessionID)
	case errors.Is(err, acp.ErrPendingPermissionConflict):
		return fmt.Errorf("%w: %s", ErrPendingPermissionConflict, sessionID)
	case errors.Is(err, acp.ErrPermissionDecisionUnsupported):
		return fmt.Errorf("%w: %s", ErrInvalidPermissionDecision, sessionID)
	default:
		return err
	}
}

func (m *Manager) resolveOrphanedPermission(
	ctx context.Context,
	sessionID string,
	interaction store.PendingInteraction,
	decision string,
) (ApprovalResult, error) {
	result, err := m.resolveOrphanedInteraction(
		ctx,
		sessionID,
		interaction,
		decision,
		resolvedByFromContext(ctx, "operator:control"),
	)
	if err != nil {
		return ApprovalResult{}, err
	}
	return ApprovalResult{
		Outcome:          result.Outcome,
		InteractionID:    result.InteractionID,
		RequestID:        result.RequestID,
		Decision:         strings.TrimSpace(decision),
		ResolvedDecision: result.Resolution,
	}, nil
}

// ResolveOrphanedClarification atomically records and queues an answer after daemon restart.
func (m *Manager) ResolveOrphanedClarification(
	ctx context.Context,
	scope toolspkg.Scope,
	requestID string,
	request toolspkg.ClarifyAnswerRequest,
) (toolspkg.ClarifyAnswerResult, error) {
	if ctx == nil {
		return toolspkg.ClarifyAnswerResult{}, errors.New("session: clarification context is required")
	}
	sessionID := strings.TrimSpace(scope.SessionID)
	target := strings.TrimSpace(requestID)
	if sessionID == "" || target == "" {
		return toolspkg.ClarifyAnswerResult{}, fmt.Errorf("%w: %s", toolspkg.ErrClarifyNotFound, target)
	}
	meta, err := m.readMetaWithContext(ctx, sessionID)
	if err != nil {
		return toolspkg.ClarifyAnswerResult{}, err
	}
	if !scope.Operator ||
		(strings.TrimSpace(scope.WorkspaceID) != "" && strings.TrimSpace(scope.WorkspaceID) != meta.WorkspaceID) ||
		(strings.TrimSpace(scope.AgentName) != "" && strings.TrimSpace(scope.AgentName) != meta.AgentName) {
		return toolspkg.ClarifyAnswerResult{}, fmt.Errorf("%w: %s", toolspkg.ErrClarifyNotFound, target)
	}
	interaction, err := m.interactionForResolution(
		ctx,
		sessionID,
		store.PendingInteractionKindClarify,
		target,
	)
	if err != nil {
		return toolspkg.ClarifyAnswerResult{}, fmt.Errorf("%w: %s", toolspkg.ErrClarifyNotFound, target)
	}
	question := toolspkg.ClarifyQuestion{
		Question: interaction.Title,
		Choices:  append([]string(nil), interaction.Payload.Choices...),
	}
	answer, err := request.Normalize(question)
	if err != nil {
		return toolspkg.ClarifyAnswerResult{}, err
	}
	resolution, err := m.resolveOrphanedInteraction(
		ctx,
		sessionID,
		interaction,
		answer.Resolution(question),
		resolvedByFromContext(ctx, "operator:control"),
	)
	if errors.Is(err, errPendingInteractionNotOrphaned) {
		return toolspkg.ClarifyAnswerResult{}, fmt.Errorf("%w: %s", toolspkg.ErrClarifyNotFound, target)
	}
	if err != nil {
		return toolspkg.ClarifyAnswerResult{}, err
	}
	winner := answerForStoredResolution(question, resolution.Resolution, answer)
	return toolspkg.ClarifyAnswerResult{
		ClarifyAnswer:  winner,
		Outcome:        resolution.Outcome,
		InteractionID:  resolution.InteractionID,
		RequestID:      resolution.RequestID,
		ResolvedAnswer: resolution.Resolution,
	}, nil
}

func answerForStoredResolution(
	question toolspkg.ClarifyQuestion,
	resolution string,
	fallback toolspkg.ClarifyAnswer,
) toolspkg.ClarifyAnswer {
	value := strings.TrimSpace(resolution)
	if value == "" || value == fallback.Resolution(question) {
		return fallback
	}
	for index, choice := range question.Choices {
		if strings.TrimSpace(choice) == value {
			selected := index
			return toolspkg.ClarifyAnswer{Choice: &selected}
		}
	}
	return toolspkg.ClarifyAnswer{Text: value}
}

func (m *Manager) resolveOrphanedInteraction(
	ctx context.Context,
	sessionID string,
	interaction store.PendingInteraction,
	resolution string,
	resolvedBy string,
) (PendingInteractionResolution, error) {
	if interaction.Status == store.PendingInteractionStatusPending {
		return PendingInteractionResolution{}, errPendingInteractionNotOrphaned
	}
	transition := store.PendingInteractionTransition{
		InteractionID: interaction.InteractionID,
		Status:        store.PendingInteractionStatusResolved,
		Resolution:    resolution,
		ResolvedBy:    resolvedBy,
		At:            m.now().UTC(),
	}
	if interaction.Status == store.PendingInteractionStatusOrphaned {
		if m.inputQueue == nil {
			return PendingInteractionResolution{}, errors.New("session: input queue is required")
		}
		generation, err := m.currentInputGeneration(ctx, sessionID)
		if err != nil {
			return PendingInteractionResolution{}, err
		}
		insert, err := m.inputQueue.PrepareQueue(inputqueue.InputRequest{
			SessionID:  sessionID,
			Text:       orphanResolutionPrompt(interaction, resolution),
			Generation: generation,
		})
		if err != nil {
			return PendingInteractionResolution{}, err
		}
		transition.OrphanInput = &insert
	}
	commit, err := m.transitionPendingInteraction(ctx, transition)
	if errors.Is(err, store.ErrSessionInputQueueFull) {
		return PendingInteractionResolution{
			Outcome:       store.PendingInteractionOutcomeQueueFull,
			InteractionID: interaction.InteractionID,
			RequestID:     interaction.ProviderRequestID,
		}, nil
	}
	if err != nil {
		return PendingInteractionResolution{}, err
	}
	resolved := interaction
	if commit.Interaction != nil {
		resolved = *commit.Interaction
	}
	return PendingInteractionResolution{
		Outcome:       commit.Outcome,
		InteractionID: resolved.InteractionID,
		RequestID:     resolved.ProviderRequestID,
		Resolution:    resolved.Resolution,
	}, nil
}

func (m *Manager) interactionForResolution(
	ctx context.Context,
	sessionID string,
	kind string,
	identity string,
) (store.PendingInteraction, error) {
	statuses := []string{
		store.PendingInteractionStatusPending,
		store.PendingInteractionStatusOrphaned,
		store.PendingInteractionStatusResolved,
		store.PendingInteractionStatusTimedOut,
		store.PendingInteractionStatusCanceled,
	}
	interactions, err := m.PendingInteractions(ctx, sessionID, statuses)
	if err != nil {
		return store.PendingInteraction{}, err
	}
	target := strings.TrimSpace(identity)
	var terminal *store.PendingInteraction
	for index := range interactions {
		interaction := interactions[index]
		if interaction.Kind != kind {
			continue
		}
		if interaction.InteractionID == target {
			return interaction, nil
		}
		if interaction.ProviderRequestID != target && interaction.TurnID != target {
			continue
		}
		if interaction.Status == store.PendingInteractionStatusPending ||
			interaction.Status == store.PendingInteractionStatusOrphaned {
			return interaction, nil
		}
		cloned := interaction
		terminal = &cloned
	}
	if terminal != nil {
		return *terminal, nil
	}
	return store.PendingInteraction{}, fmt.Errorf("%w: %s", store.ErrPendingInteractionNotFound, target)
}

func orphanResolutionPrompt(interaction store.PendingInteraction, resolution string) string {
	title := strings.TrimSpace(interaction.Title)
	if title == "" {
		title = strings.TrimSpace(interaction.Kind) + " request " + strings.TrimSpace(interaction.ProviderRequestID)
	}
	return title + "\nResponse: " + strings.TrimSpace(resolution)
}
