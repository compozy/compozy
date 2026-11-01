package agentinstance

import (
	"context"
	"errors"
	"github.com/compozy/compozy/internal/hooks"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

type ListQuery struct {
	ProfileID, WorkspaceID, AfterID string
	AllWorkspaces                   bool
	Limit                           int
}

type InstanceStore interface {
	Get(context.Context, string) (Instance, error)
	PublishMissingMain(context.Context, string, string) (Instance, error)
	ReconciliationPage(context.Context, string, int) ([]Instance, error)
	Adopt(context.Context, Instance, string, int) (Instance, error)
	AdmitCreate(context.Context, Instance, int) (Instance, bool, error)
	BindMain(context.Context, string, string) (Instance, error)
	DeleteUnstarted(context.Context, string) error
	Pause(context.Context, string, string) (Instance, error)
	Resume(context.Context, string, int) (Instance, error)
	Archive(context.Context, string) (Instance, error)
	List(context.Context, ListQuery) ([]Instance, error)
}

type CreateRequest struct {
	Agent, Workspace, Profile string
	Actor                     workspaceaccess.ActorRef
}

// Admission resolves authenticated lineage, definition and intersected permissions at the daemon boundary.
type Admission interface {
	ResolveCreate(context.Context, CreateRequest) (Instance, error)
	ValidateResume(context.Context, Instance) error
	ResolveAdopt(context.Context, AdoptRequest) (Instance, error)
}

type StartMainSession struct{ Instance Instance }

type MainSessionStarter interface {
	Start(context.Context, StartMainSession) (string, error)
	Stop(context.Context, string) error
}

type Service struct {
	hooks     LifecycleHooks
	store     InstanceStore
	admission Admission
	starter   MainSessionStarter
	limit     int
	now       func() time.Time
}

type Option func(*Service)

func WithStore(store InstanceStore) Option     { return func(s *Service) { s.store = store } }
func WithAdmission(admission Admission) Option { return func(s *Service) { s.admission = admission } }
func WithMainSessions(starter MainSessionStarter) Option {
	return func(s *Service) { s.starter = starter }
}
func WithActiveLimit(limit int) Option { return func(s *Service) { s.limit = limit } }
func NewService(options ...Option) (*Service, error) {
	s := &Service{now: time.Now}
	for _, option := range options {
		option(s)
	}
	if s.store == nil || s.admission == nil || s.starter == nil || s.limit <= 0 {
		return nil, errors.New("agentinstance: store, admission, main sessions and positive limit are required")
	}
	return s, nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (Instance, bool, error) {
	input, err := s.admission.ResolveCreate(ctx, req)
	if err != nil {
		return Instance{}, false, err
	}
	instance, created, err := s.store.AdmitCreate(ctx, input, s.limit)
	if err != nil || !created {
		return instance, false, err
	}
	sessionID, err := s.starter.Start(ctx, StartMainSession{Instance: instance})
	if err != nil {
		if sessionID == "" {
			err = errors.Join(err, s.store.DeleteUnstarted(context.WithoutCancel(ctx), instance.ID))
		}
		return instance, true, err
	}
	if sessionID == "" {
		return instance, true, errors.Join(
			errors.New("agentinstance: main session starter returned an empty session id"),
			s.store.DeleteUnstarted(context.WithoutCancel(ctx), instance.ID),
		)
	}
	instance, err = s.store.BindMain(ctx, instance.ID, sessionID)
	if err == nil {
		s.notifyLifecycle(ctx, hooks.HookAgentInstanceCreated, instance)
	}
	return instance, true, err
}

func (s *Service) Pause(ctx context.Context, id, reason string) (Instance, error) {
	switch reason {
	case "manual", "fatigue", "definition_not_stays_on", "definition_missing":
	default:
		return Instance{}, errors.New("agentinstance: invalid pause reason")
	}
	before, err := s.store.Get(ctx, id)
	if err != nil {
		return Instance{}, err
	}
	instance, err := s.store.Pause(ctx, id, reason)
	if err == nil && before.State != instance.State {
		s.notifyLifecycle(ctx, hooks.HookAgentInstancePaused, instance)
	}
	return instance, err
}
func (s *Service) Resume(ctx context.Context, id string) (Instance, error) {
	instance, err := s.store.Get(ctx, id)
	if err != nil {
		return Instance{}, err
	}
	if err := s.admission.ValidateResume(ctx, instance); err != nil {
		return Instance{}, err
	}
	resumed, err := s.store.Resume(ctx, id, s.limit)
	if err == nil && instance.State != resumed.State {
		s.notifyLifecycle(ctx, hooks.HookAgentInstanceResumed, resumed)
	}
	return resumed, err
}
func (s *Service) Archive(ctx context.Context, id string) error {
	before, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	instance, err := s.store.Archive(ctx, id)
	if err != nil {
		return err
	}
	if before.ArchivedAt == nil {
		s.notifyLifecycle(ctx, hooks.HookAgentInstanceArchived, instance)
	}
	if instance.MainSessionID != "" {
		return s.starter.Stop(ctx, instance.MainSessionID)
	}
	return nil
}
func (s *Service) List(ctx context.Context, q ListQuery) ([]Instance, error) {
	return s.store.List(ctx, q)
}

type AdoptRequest struct {
	CreateRequest
	SessionID string
}

func (s *Service) Adopt(ctx context.Context, req AdoptRequest) (Instance, error) {
	if !req.Actor.Operator {
		return Instance{}, ErrForbidden
	}
	input, err := s.admission.ResolveAdopt(ctx, req)
	if err != nil {
		return Instance{}, err
	}
	instance, err := s.store.Adopt(ctx, input, req.SessionID, s.limit)
	if err == nil {
		s.notifyLifecycle(ctx, hooks.HookAgentInstanceAdopted, instance)
	}
	return instance, err
}

func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}
