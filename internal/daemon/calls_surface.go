package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
)

type callSurfaceService struct {
	*callspkg.Service
	sessions SessionManager
}

func newCallSurfaceService(service *callspkg.Service, sessions SessionManager) *callSurfaceService {
	return &callSurfaceService{Service: service, sessions: sessions}
}

func (s *callSurfaceService) ResolveOperatorCaller(
	ctx context.Context,
	scope callspkg.CallScope,
	actor callspkg.Actor,
) (participation.OwnerRef, error) {
	if s == nil || s.Service == nil || s.sessions == nil {
		return participation.OwnerRef{}, errors.New("daemon: calls operator surface is unavailable")
	}
	if strings.TrimSpace(actor.Kind) != "human" || strings.TrimSpace(actor.ID) == "" {
		return participation.OwnerRef{}, &callspkg.Error{
			Code: callspkg.CodeSettlementDenied, Message: "operator caller requires a human actor",
		}
	}
	scope.ProfileID = strings.TrimSpace(scope.ProfileID)
	scope.WorkspaceID = strings.TrimSpace(scope.WorkspaceID)
	if scope.ProfileID == "" {
		return participation.OwnerRef{}, &callspkg.Error{
			Code: callspkg.CodeValidation, Message: "operator caller profile is required",
		}
	}
	sessionID := operatorCallerSessionID(scope)
	if err := s.ensureOperatorCallerSession(ctx, scope, sessionID); err != nil {
		return participation.OwnerRef{}, err
	}
	binding, err := s.Service.ResolveOperatorCaller(ctx, callspkg.OperatorCallerBinding{
		ProfileID: scope.ProfileID, Scope: scope.Scope, WorkspaceID: scope.WorkspaceID, SessionID: sessionID,
	})
	if err != nil {
		return participation.OwnerRef{}, fmt.Errorf("daemon: bind operator caller: %w", err)
	}
	return participation.OwnerRef{
		Kind: participation.OwnerKindSession, ID: binding.SessionID, WorkspaceID: scope.WorkspaceID,
	}, nil
}

func (s *callSurfaceService) ensureOperatorCallerSession(
	ctx context.Context,
	scope callspkg.CallScope,
	sessionID string,
) error {
	info, err := s.sessions.Status(ctx, sessionID)
	if err == nil {
		if info.DrainingAt != nil {
			if info.State != session.StateStopped {
				return fmt.Errorf("daemon: operator caller %q is still draining", sessionID)
			}
			if reopenErr := s.Service.ReopenOperatorCaller(ctx, sessionID); reopenErr != nil {
				return fmt.Errorf("daemon: reopen operator caller %q: %w", sessionID, reopenErr)
			}
		}
		if info.State == session.StateStopped {
			if _, resumeErr := s.sessions.Resume(ctx, sessionID); resumeErr != nil {
				return fmt.Errorf("daemon: resume operator caller %q: %w", sessionID, resumeErr)
			}
		}
		return nil
	}
	if !errors.Is(err, session.ErrSessionNotFound) {
		return fmt.Errorf("daemon: inspect operator caller %q: %w", sessionID, err)
	}
	acceptance, ok := s.sessions.(interface {
		CreateAccepted(context.Context, session.CreateAcceptedOpts) (*session.Info, error)
	})
	if !ok {
		return errors.New("daemon: session manager cannot accept an operator caller")
	}
	_, createErr := acceptance.CreateAccepted(ctx, session.CreateAcceptedOpts{
		RuntimeFree: true,
		Session: session.CreateOpts{
			DesiredSessionID: sessionID,
			Global:           scope.Scope == callspkg.ScopeGlobal,
			ProfileID:        scope.ProfileID,
			Workspace:        scope.WorkspaceID,
			Name:             "Operator calls",
			Type:             session.SessionTypeUser,
			DisableSandbox:   true,
		},
	})
	if createErr == nil {
		return nil
	}
	if _, statusErr := s.sessions.Status(ctx, sessionID); statusErr == nil {
		return nil
	}
	return fmt.Errorf("daemon: create operator caller %q: %w", sessionID, createErr)
}

func operatorCallerSessionID(scope callspkg.CallScope) string {
	identity := strings.Join([]string{
		strings.TrimSpace(scope.ProfileID), string(scope.Scope), strings.TrimSpace(scope.WorkspaceID),
	}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return "ses_operator_" + hex.EncodeToString(digest[:12])
}
