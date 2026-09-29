package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

var (
	errDeriveRuntimeRouteExclusive = fmt.Errorf("%w: runtime and route are mutually exclusive", session.ErrValidation)
	errDerivePartialFences         = fmt.Errorf(
		"%w: set all transcript fences together or omit all three", session.ErrValidation,
	)
	errDeriveUnavailable = errors.New("api: session continue is unavailable")
)

// SessionDeriveManager continues sessions and previews what a derive would carry.
type SessionDeriveManager interface {
	ContinueSession(ctx context.Context, opts session.ContinueSessionOpts) (session.DeriveResult, error)
	ForkSession(ctx context.Context, opts session.ForkSessionOpts) (session.DeriveResult, error)
	DerivePreview(
		ctx context.Context,
		workspaceID string,
		sourceSessionID string,
		messageID string,
	) (session.DerivePreview, error)
}

// ContinueSession starts a new session for another agent with the source conversation carried over.
func (h *BaseHandlers) ContinueSession(c *gin.Context) {
	var req contract.ContinueSessionRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondDeriveError(c, http.StatusBadRequest, invalidDeriveRequest(
			fmt.Errorf("%s: decode continue request: %w", h.transportName(), err),
		))
		return
	}
	if err := validateContinueSessionRequest(req); err != nil {
		h.respondDeriveError(c, http.StatusBadRequest, err)
		return
	}
	scope, sessionID, deriver, ok := h.deriveRoute(c)
	if !ok {
		return
	}
	result, err := deriver.ContinueSession(c.Request.Context(), session.ContinueSessionOpts{
		SourceSessionID: sessionID,
		WorkspaceID:     scope.SessionWorkspaceID(),
		AgentName:       strings.TrimSpace(req.AgentName),
		Runtime:         deriveRuntimeFromPayload(req.Runtime),
		Route:           req.Route,
		Name:            req.Name,
		Message:         req.Message,
		IdempotencyKey:  strings.TrimSpace(req.IdempotencyKey),
		Fences: session.DeriveFences{
			ExpectedEpoch: req.ExpectedEpoch, ExpectedGeneration: req.ExpectedGeneration,
			ExpectedMaxSequence: req.ExpectedMaxSequence,
		},
	})
	h.respondDerive(c, result, err)
}

// ForkSession forks the session with the same agent into a new session.
func (h *BaseHandlers) ForkSession(c *gin.Context) {
	var req contract.ForkSessionRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondDeriveError(c, http.StatusBadRequest, invalidDeriveRequest(
			fmt.Errorf("%s: decode fork request: %w", h.transportName(), err),
		))
		return
	}
	if err := validateForkSessionRequest(req); err != nil {
		h.respondDeriveError(c, http.StatusBadRequest, err)
		return
	}
	scope, sessionID, deriver, ok := h.deriveRoute(c)
	if !ok {
		return
	}
	result, err := deriver.ForkSession(c.Request.Context(), session.ForkSessionOpts{
		SourceSessionID: sessionID,
		WorkspaceID:     scope.SessionWorkspaceID(),
		MessageID:       strings.TrimSpace(req.MessageID),
		Name:            req.Name,
		IdempotencyKey:  strings.TrimSpace(req.IdempotencyKey),
		Fences: session.DeriveFences{
			ExpectedEpoch: req.ExpectedEpoch, ExpectedGeneration: req.ExpectedGeneration,
			ExpectedMaxSequence: req.ExpectedMaxSequence,
		},
	})
	h.respondDerive(c, result, err)
}

// respondDerive writes a continue/fork outcome: 201 for a new child, 200 for a replayed receipt.
// A failure after the child was committed (first-message admission, projection) keeps the
// child reachable: the error carries child_session_id.
func (h *BaseHandlers) respondDerive(c *gin.Context, result session.DeriveResult, err error) {
	if err != nil {
		err = deriveFailure(err, committedDeriveChildID(result))
		h.respondDeriveError(c, StatusForSessionError(err), err)
		return
	}
	response, err := h.sessionDeriveResponse(c.Request.Context(), result)
	if err != nil {
		err = deriveFailure(err, committedDeriveChildID(result))
		h.respondDeriveError(c, StatusForSessionError(err), err)
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	c.JSON(status, response)
}

// deriveRoute resolves the workspace scope and the path's source session id without
// reading the source: Status repairs an inactive session's metadata, and a derive or
// preview never writes the source (ADR-007). The manager validates source ownership
// with its non-repairing snapshot read, after replaying a matching receipt, so a retry
// still returns its recorded outcome once the source was deleted. The child inherits
// the source's profile inside the manager.
func (h *BaseHandlers) deriveRoute(c *gin.Context) (workspaceScope, string, SessionDeriveManager, bool) {
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return workspaceScope{}, "", nil, false
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		h.respondDeriveError(c, http.StatusBadRequest, invalidDeriveRequest(
			fmt.Errorf("%s: session_id path is required", h.transportName()),
		))
		return workspaceScope{}, "", nil, false
	}
	deriver, ok := h.Sessions.(SessionDeriveManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errDeriveUnavailable)
		return workspaceScope{}, "", nil, false
	}
	return scope, sessionID, deriver, true
}

// PreviewSessionDerive reports what a continue or fork of the session would carry; it never writes.
func (h *BaseHandlers) PreviewSessionDerive(c *gin.Context) {
	scope, sessionID, deriver, ok := h.deriveRoute(c)
	if !ok {
		return
	}
	preview, err := deriver.DerivePreview(
		c.Request.Context(),
		scope.SessionWorkspaceID(),
		sessionID,
		strings.TrimSpace(c.Query("message_id")),
	)
	if err != nil {
		h.respondDeriveError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, SessionDerivePreviewPayload(preview))
}

func validateContinueSessionRequest(req contract.ContinueSessionRequest) error {
	switch {
	case strings.TrimSpace(req.AgentName) == "":
		return fmt.Errorf("%w: agent_name is required", session.ErrValidation)
	case strings.TrimSpace(req.IdempotencyKey) == "":
		return fmt.Errorf("%w: idempotency_key is required", session.ErrValidation)
	case req.Route < 0:
		return fmt.Errorf("%w: route must be a positive 1-based index", session.ErrValidation)
	case req.Runtime != nil && req.Route > 0:
		return errDeriveRuntimeRouteExclusive
	}
	return validateDeriveFences(req.ExpectedEpoch, req.ExpectedGeneration, req.ExpectedMaxSequence)
}

func validateForkSessionRequest(req contract.ForkSessionRequest) error {
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return fmt.Errorf("%w: idempotency_key is required", session.ErrValidation)
	}
	return validateDeriveFences(req.ExpectedEpoch, req.ExpectedGeneration, req.ExpectedMaxSequence)
}

func validateDeriveFences(epoch, generation, maxSequence *int64) error {
	set := 0
	for _, fence := range []*int64{epoch, generation, maxSequence} {
		if fence != nil {
			set++
		}
	}
	if set != 0 && set != 3 {
		return errDerivePartialFences
	}
	return nil
}

func deriveRuntimeFromPayload(payload *contract.PromptRuntimeSelectionPayload) *session.DeriveRuntime {
	selection := contract.PromptRuntimeSelectionFromPayload(payload)
	if selection == nil {
		return nil
	}
	return &session.DeriveRuntime{
		Provider:        selection.Provider,
		Model:           selection.Model,
		ReasoningEffort: selection.ReasoningEffort,
		Speed:           selection.Speed,
		ACPOptions:      selection.ACPOptions,
	}
}

func (h *BaseHandlers) sessionDeriveResponse(
	ctx context.Context,
	result session.DeriveResult,
) (contract.SessionDeriveResponse, error) {
	response := contract.SessionDeriveResponse{Derived: SessionDerivedPayloadFromResult(result)}
	if result.Child == nil {
		return response, nil
	}
	payload, err := h.sessionPayloadWithOptionalHealth(ctx, result.Child, false)
	if err != nil {
		return contract.SessionDeriveResponse{}, err
	}
	response.Session = &payload
	return response, nil
}

// SessionDerivedPayloadFromResult projects one derive outcome onto the public payload.
func SessionDerivedPayloadFromResult(result session.DeriveResult) contract.SessionDerivedPayload {
	messageCount := result.ReplayMessageCount
	replayBytes := result.ReplayBytes
	return contract.SessionDerivedPayload{
		Kind:                 string(result.Kind),
		SourceSessionID:      result.SourceSessionID,
		OriginAgentName:      result.OriginAgentName,
		OriginMessageID:      result.OriginMessageID,
		ThroughTurnID:        result.ThroughTurnID,
		Seed:                 string(result.Seed),
		NativeState:          result.NativeState,
		ACPSessionID:         result.ACPSessionID,
		NativeForkError:      result.NativeForkError,
		ReplayMessageCount:   &messageCount,
		ReplayBytes:          &replayBytes,
		Truncated:            result.Truncated,
		OmittedCount:         result.OmittedCount,
		SourceTurnInProgress: result.SourceTurnInProgress,
		FirstPrompt:          result.FirstPrompt,
		Replayed:             result.Replayed,
		ChildDeleted:         result.ChildDeleted,
		ChildSessionID:       result.ChildSessionID,
	}
}

// SessionDerivePreviewPayload projects a derive preview onto the public payload.
func SessionDerivePreviewPayload(preview session.DerivePreview) contract.SessionDerivePreviewResponse {
	payload := contract.SessionDerivePreviewResponse{
		MessageCount:         preview.MessageCount,
		SourceMessageCount:   preview.SourceMessageCount,
		ReplayBytes:          preview.ReplayBytes,
		Truncated:            preview.Truncated,
		OmittedCount:         preview.OmittedCount,
		SourceTurnInProgress: preview.SourceTurnInProgress,
		NativeForkPossible:   preview.NativeForkPossible,
		Transcript: contract.SessionTranscriptFences{
			Epoch: preview.Epoch, Generation: preview.Generation, MaxSequence: preview.MaxSequence,
		},
	}
	if preview.Cut != nil {
		payload.Cut = &contract.SessionDeriveCutPayload{
			MessageID: preview.Cut.MessageID, TurnID: preview.Cut.TurnID, TurnSettled: preview.Cut.TurnSettled,
		}
	}
	return payload
}

func sessionDerivationPayload(derivation *store.SessionDerivation) *contract.SessionDerivationPayload {
	if derivation == nil || !derivation.Kind.Derived() {
		return nil
	}
	payload := &contract.SessionDerivationPayload{
		Kind:            derivation.Kind,
		SourceSessionID: strings.TrimSpace(derivation.SourceSessionID),
		Seed:            strings.TrimSpace(derivation.Seed),
		FirstPrompt:     strings.TrimSpace(derivation.FirstPrompt.State),
	}
	if native := derivation.Native; native != nil {
		payload.NativeState = strings.TrimSpace(native.State)
		payload.NativeForkError = strings.TrimSpace(native.Error)
	}
	return payload
}
