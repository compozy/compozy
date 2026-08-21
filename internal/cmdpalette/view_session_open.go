package cmdpalette

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const defaultViewOpenBudget = 5 * time.Second

// OpenSession creates one client- and extension-instance-bound program session.
func (s *Service) OpenSession(
	ctx context.Context,
	request ViewSessionOpenRequest,
) (ViewSessionOpenResult, error) {
	if err := s.validateViewSessionOpen(ctx, request); err != nil {
		return ViewSessionOpenResult{}, err
	}
	client, err := s.resolveAttachedViewClient(ctx, request)
	if err != nil {
		return ViewSessionOpenResult{}, err
	}
	request.Client = client
	descriptor, err := s.ResolveView(ctx, request.ProfileLens, request.Workspace, request.View)
	if err != nil {
		return ViewSessionOpenResult{}, err
	}
	if !descriptor.Program || descriptor.Extension == "" {
		return ViewSessionOpenResult{}, viewValidationError(
			"view",
			"view %q is not programmable",
			descriptor.ID,
		)
	}

	session := newViewSession(ctx, request, descriptor)
	validated, err := s.openInitialViewFrame(ctx, request, descriptor, session)
	if err != nil {
		return ViewSessionOpenResult{}, err
	}
	s.viewSessionMu.Lock()
	s.viewSessions[session.id] = session
	s.acceptViewFrameLocked(session, validated)
	firstFrame := cloneViewFrame(session.lastFrame)
	s.viewSessionMu.Unlock()
	s.emitViewSessionEvent(ctx, EventViewSessionOpened, session)

	return ViewSessionOpenResult{
		ProfileLens: request.ProfileLens,
		Token: SessionToken{
			ViewSession: session.id,
			StreamToken: session.streamToken,
		},
		FirstFrame: firstFrame,
	}, nil
}

func (s *Service) validateViewSessionOpen(ctx context.Context, request ViewSessionOpenRequest) error {
	if ctx == nil {
		return errors.New("cmd palette view: context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.viewPrograms == nil {
		return errors.New("cmd palette view: program provider is unavailable")
	}
	if request.Workspace == "" {
		return errors.New("cmd palette view: workspace is required")
	}
	if err := request.ProfileLens.Validate(); err != nil {
		return err
	}
	if s.clients == nil || strings.TrimSpace(request.AttachmentToken) == "" {
		return ErrClientUnauthorized
	}
	return nil
}

func newViewSession(
	ctx context.Context,
	request ViewSessionOpenRequest,
	descriptor ViewDescriptor,
) *viewSession {
	sessionCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	return &viewSession{
		id: "vs_" + uuid.NewString(), streamToken: "vst_" + uuid.NewString(),
		profileLens: request.ProfileLens,
		workspace:   request.Workspace, client: request.Client, view: descriptor.ID,
		extension: descriptor.Extension,
		kind:      descriptor.Kind, ctx: sessionCtx, cancel: cancel,
		handlers: make(map[string]uint64), coalescibleHandlers: make(map[string]struct{}),
		ackedEffects:        make(map[string]struct{}),
		subscribers:         make(map[uint64]chan ViewFrame),
		rejectedGenerations: make(map[uint64]struct{}),
		actions:             make(map[uint64]viewEventFlight),
	}
}

func (s *Service) openInitialViewFrame(
	ctx context.Context,
	request ViewSessionOpenRequest,
	descriptor ViewDescriptor,
	session *viewSession,
) (ViewFrame, error) {
	openCtx, cancel := context.WithTimeout(session.ctx, defaultViewOpenBudget)
	defer cancel()
	frame, generation, err := s.viewPrograms.OpenProgram(openCtx, descriptor.Extension, ViewOpenRequest{
		ViewSession: session.id,
		View:        descriptor.ID,
		ProfileLens: request.ProfileLens,
		Workspace:   request.Workspace,
		Client:      request.Client,
		Args:        cloneAnyMap(request.Args),
	})
	if err != nil {
		return ViewFrame{}, s.failViewSessionOpen(ctx, session, "open_failed", err)
	}
	session.instanceGeneration = generation
	validated, err := validateViewFrame(descriptor.Kind, frame)
	if err != nil {
		return ViewFrame{}, s.failViewSessionOpen(ctx, session, "invalid_first_frame", err)
	}
	if validated.ViewSession != session.id || validated.Generation != 0 || validated.InReplyTo != 0 {
		err = errors.New("cmd palette view: first frame must match the session with generation and reply zero")
		return ViewFrame{}, s.failViewSessionOpen(ctx, session, "invalid_first_frame", err)
	}
	return validated, nil
}

func (s *Service) failViewSessionOpen(
	ctx context.Context,
	session *viewSession,
	reason string,
	cause error,
) error {
	session.cancel()
	cleanupErr := s.closeViewProgram(
		ctx, session.profileLens, session.workspace, session.extension, ViewCloseRequest{
			ViewSession: session.id,
			ProfileLens: session.profileLens,
			Reason:      reason,
		},
	)
	return errors.Join(cause, cleanupErr)
}

func (s *Service) resolveAttachedViewClient(
	ctx context.Context,
	request ViewSessionOpenRequest,
) (ClientID, error) {
	if request.Client != "" {
		if err := s.clients.Authorize(
			ctx,
			request.Workspace,
			request.Client,
			request.AttachmentToken,
		); err != nil {
			return "", fmt.Errorf("%w: %v", ErrClientUnauthorized, err)
		}
		return request.Client, nil
	}
	clients, err := s.clients.Clients(ctx, request.Workspace)
	if err != nil {
		return "", fmt.Errorf("cmd palette view: list attached clients: %w", err)
	}
	var matched ClientID
	for _, candidate := range clients {
		if err := s.clients.Authorize(
			ctx,
			request.Workspace,
			candidate.ID,
			request.AttachmentToken,
		); err != nil {
			continue
		}
		if matched != "" {
			return "", ErrClientUnauthorized
		}
		matched = candidate.ID
	}
	if matched == "" {
		return "", ErrClientUnauthorized
	}
	return matched, nil
}
