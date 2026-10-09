package session

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

type subagentSnapshot struct {
	Info      *Info
	TurnID    string
	Active    bool
	Queued    int
	UserSteer bool
	CanSteer  bool
}

type subagentRuntime interface {
	Snapshot(context.Context, string) (subagentSnapshot, error)
	Resolve(context.Context, *Info, SubagentTarget) (SubagentTarget, error)
	Capabilities(context.Context, *Info) ([]SubagentAgentOption, []SubagentProviderOption, error)
	Spawn(context.Context, SpawnOpts) (string, error)
	Admit(context.Context, store.SessionSubagent, string) error
	HasAdmission(context.Context, store.SessionSubagent) (bool, error)
	Stop(context.Context, string) error
	Result(context.Context, string) (string, error)
	QueueWake(context.Context, store.SessionSubagentWake, []store.SessionSubagent) (string, error)
	CancelWake(context.Context, store.SessionSubagentWake) error
	Steer(context.Context, string, string, string, string) (acp.SteerResult, error)
	PublishParent(context.Context, string)
	SettleParent(context.Context, string) error
}

// SubagentSettledDispatcher observes the one successful terminal transition.
type SubagentSettledDispatcher interface {
	DispatchSubagentSettled(context.Context, store.SessionSubagent) error
}

type SubagentOption func(*subagentService)

func WithSubagentResultLimit(get func() int) SubagentOption {
	return func(s *subagentService) {
		if get != nil {
			s.resultLimit = get
		}
	}
}

func WithSubagentSettledDispatcher(d SubagentSettledDispatcher) SubagentOption {
	return func(s *subagentService) { s.settled = d }
}

type subagentService struct {
	store       store.SubagentStore
	runtime     subagentRuntime
	ctx         context.Context
	now         func() time.Time
	newID       IDGenerator
	launch      func(func())
	logger      *slog.Logger
	resultLimit func() int
	settled     SubagentSettledDispatcher
	mu          sync.Mutex
	parents     map[string]*sync.Mutex
	flights     map[string]chan struct{}
	subscribers map[string]map[*subagentSubscription]struct{}
	progress    map[string]*subagentProgress
}

var _ SubagentService = (*subagentService)(nil)
var _ SubagentUpdateSubscriber = (*subagentService)(nil)

func NewSubagentService(db store.SubagentStore, manager *Manager, opts ...SubagentOption) (SubagentService, error) {
	if db == nil || manager == nil {
		return nil, errors.New("session: subagent store and manager are required")
	}
	s := &subagentService{
		store: db, runtime: managerSubagentRuntime{manager}, ctx: manager.fallbackLifecycleContext(),
		now: manager.now, newID: newULIDGenerator("wake"), launch: manager.startTrackedPromptTask,
		logger: manager.logger, resultLimit: func() int { return 60000 },
		parents: make(map[string]*sync.Mutex), flights: make(map[string]chan struct{}),
		subscribers: make(map[string]map[*subagentSubscription]struct{}), progress: make(map[string]*subagentProgress),
	}
	if s.logger == nil {
		s.logger = slog.Default()
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

func (m *Manager) SetSubagentService(s SubagentService) { m.mu.Lock(); m.subagents = s; m.mu.Unlock() }
func (m *Manager) subagentService() SubagentService {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.subagents
}

func (s *subagentService) lock(parent string) func() {
	s.mu.Lock()
	lock := s.parents[parent]
	if lock == nil {
		lock = new(sync.Mutex)
		s.parents[parent] = lock
	}
	s.mu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func (s *subagentService) caller(ctx context.Context, caller SubagentCaller) (subagentSnapshot, error) {
	snap, err := s.runtime.Snapshot(ctx, caller.SessionID)
	if err != nil {
		return subagentSnapshot{}, err
	}
	if snap.Info == nil || snap.Info.WorkspaceID != caller.WorkspaceID || !snap.Active ||
		snap.Info.State != StateActive ||
		snap.TurnID != caller.TurnID ||
		caller.TurnID == "" {
		return subagentSnapshot{}, ErrSubagentParentNotActive
	}
	return snap, nil
}

func (s *subagentService) Capabilities(ctx context.Context, caller SubagentCaller) (SubagentCapabilities, error) {
	snap, err := s.caller(ctx, caller)
	if err != nil {
		return SubagentCapabilities{}, err
	}
	agents, providers, err := s.runtime.Capabilities(ctx, snap.Info)
	if err != nil {
		return SubagentCapabilities{}, err
	}
	summaries, err := s.store.Summaries(ctx, []string{caller.SessionID})
	if err != nil {
		return SubagentCapabilities{}, err
	}
	depth, err := s.depth(ctx, caller.SessionID)
	if err != nil {
		return SubagentCapabilities{}, err
	}
	return SubagentCapabilities{
		ParentSessionID: caller.SessionID,
		Inherited:       subagentInherited(snap.Info),
		PermissionMode:  permissionMode(snap.Info),
		Depth:           depth,
		Live:            summaries[caller.SessionID].Live,
		Agents:          agents,
		Providers:       providers,
	}, nil
}

func (s *subagentService) depth(ctx context.Context, parent string) (int, error) {
	row, err := s.store.GetSubagentByChild(ctx, parent)
	if errors.Is(err, store.ErrSubagentNotFound) {
		return 0, nil
	}
	return row.Depth, err
}

func (s *subagentService) Get(ctx context.Context, workspace, id string) (Subagent, error) {
	row, err := s.store.GetSubagent(ctx, workspace, id)
	if errors.Is(err, store.ErrSubagentNotFound) {
		return Subagent{}, ErrSubagentNotFound
	}
	return presentSubagent(row), err
}
func (s *subagentService) List(ctx context.Context, q store.SubagentListQuery) (store.SubagentPage, error) {
	return s.store.ListSubagents(ctx, q)
}
func (s *subagentService) Summaries(ctx context.Context, ids []string) (map[string]store.SubagentSummary, error) {
	return s.store.Summaries(ctx, ids)
}

func (s *subagentService) OnNativeToolEvent(_ context.Context, _ string, ev NativeSubagentEvent) error {
	if ev.ProviderToolCallID == "" {
		return nil
	}
	return errors.New("session: provider-native subagent event adapter is not installed")
}

func (s *subagentService) logError(ctx context.Context, operation string, err error) {
	if err != nil {
		s.logger.ErrorContext(ctx, "subagent."+operation, "error", err)
	}
}
