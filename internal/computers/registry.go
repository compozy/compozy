package computers

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"sync"

	"github.com/jonboulle/clockwork"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
)

type Option func(*Registry)

func WithClock(clock clockwork.Clock) Option { return func(r *Registry) { r.clock = clock } }
func WithLogger(logger *slog.Logger) Option  { return func(r *Registry) { r.logger = logger } }
func WithSSHForward(resolve ForwardResolver) Option {
	return func(r *Registry) { r.forward = resolve }
}

func WithSSHUninstall(uninstall func(context.Context, store.ComputerRoute) error) Option {
	return func(r *Registry) { r.uninstall = uninstall }
}

func WithManagedSSH(build func(*Registry) *sshsetup.Manager) Option {
	return func(r *Registry) {
		r.setups = build(r)
		r.uninstall = r.setups.Uninstall
	}
}

type Registry struct {
	store     store.ComputerStore
	account   Account
	cloud     Cloud
	forward   ForwardResolver
	uninstall func(context.Context, store.ComputerRoute) error
	setups    *sshsetup.Manager
	routeHTTP func(remoteproto.RouteKind) *http.Client
	clock     clockwork.Clock
	logger    *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}

	mu    sync.Mutex
	scope *registryScope
}

type registryScope struct {
	registry  *Registry
	ctx       context.Context
	cancel    context.CancelFunc
	identity  context.Context
	accountID string

	mu      sync.Mutex
	workers sync.WaitGroup
	entries map[remoteproto.ComputerID]*registryEntry
	events  eventRing
}

type registryEntry struct {
	view  View
	link  *connection
	run   *supervisor
	fault error
}

func New(ctx context.Context, store store.ComputerStore, accounts Account, cloud Cloud, opts ...Option) *Registry {
	// #nosec G118 -- the registry owns cancel until Close.
	lifetime, cancel := context.WithCancel(context.WithoutCancel(ctx))
	r := &Registry{
		ctx: lifetime, cancel: cancel, done: make(chan struct{}),
		store: store, account: accounts, cloud: cloud,
		clock: clockwork.NewRealClock(), logger: slog.Default(), routeHTTP: routeClient,
	}
	for _, opt := range opts {
		opt(r)
	}
	go r.watchIdentity()
	return r
}

func (r *Registry) Close() {
	r.cancel()
	if r.setups != nil {
		r.setups.Close()
	}
	r.mu.Lock()
	if r.scope != nil {
		r.scope.cancel()
	}
	r.mu.Unlock()
	<-r.done
}

func (r *Registry) watchIdentity() {
	defer close(r.done)
	defer r.dropScope(nil)
	for r.ctx.Err() == nil {
		lifetime, identity := r.account.IdentityLifetime()
		if identity.State == account.SignedIn && identity.AccountID != "" {
			s, err := r.activate(lifetime, identity)
			if err == nil {
				r.watchScope(s)
				r.dropScope(s)
				continue
			}
			select {
			case <-r.ctx.Done():
			case <-lifetime.Done():
			case <-r.clock.After(healthInterval):
			}
			continue
		}
		select {
		case <-r.ctx.Done():
		case <-lifetime.Done():
		}
	}
}

func (r *Registry) dropScope(expected *registryScope) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.scope != nil && (expected == nil || r.scope == expected) {
		r.scope.close()
		r.scope = nil
	}
}

func (r *Registry) watchScope(scope *registryScope) {
	ticker := r.clock.NewTicker(betterRouteInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-scope.ctx.Done():
			return
		case <-ticker.Chan():
			scope.refreshRoutes()
		}
	}
}

func (r *Registry) current() (*registryScope, error) {
	lifetime, identity := r.account.IdentityLifetime()
	if identity.State != account.SignedIn || identity.AccountID == "" {
		return nil, account.ErrSignInRequired
	}
	return r.activate(lifetime, identity)
}

func (r *Registry) activate(lifetime context.Context, identity account.Installation) (*registryScope, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ctx.Err() != nil || lifetime.Err() != nil {
		return nil, account.ErrSignInRequired
	}
	if r.scope != nil {
		if r.scope.identity == lifetime {
			return r.scope, nil
		}
		r.scope.close()
		r.scope = nil
	}
	// #nosec G118 -- the scope owns cancel until the account generation or registry closes.
	ctx, cancel := context.WithCancel(lifetime)
	s := &registryScope{
		registry: r, ctx: ctx, cancel: cancel, identity: lifetime, accountID: identity.AccountID,
		entries: make(map[remoteproto.ComputerID]*registryEntry), events: newEventRing(identity.AuthGeneration),
	}
	stored, err := r.store.ControllerComputers(ctx, s.accountID)
	if err != nil {
		cancel()
		return nil, err
	}
	s.mu.Lock()
	for _, computer := range stored {
		entry := &registryEntry{view: initialView(computer, len(computer.Routes) > 0)}
		s.entries[computer.ID] = entry
		s.start(entry)
	}
	s.mu.Unlock()
	r.scope = s
	return s, nil
}

func (s *registryScope) close() {
	s.cancel()
	s.mu.Lock()
	for _, entry := range s.entries {
		s.stop(entry)
	}
	clear(s.entries)
	s.mu.Unlock()
	s.workers.Wait()
}

func (s *registryScope) device(
	ctx context.Context, run func(context.Context, cloudclient.Credentials) error,
) error {
	req, done := s.operation(ctx)
	defer done()
	if s.ctx.Err() != nil {
		return account.ErrSignInRequired
	}
	return s.registry.account.UseDevice(req, func(ctx context.Context, credentials cloudclient.Credentials) error {
		if s.ctx.Err() != nil {
			return account.ErrSignInRequired
		}
		return run(ctx, credentials)
	})
}

func (s *registryScope) operation(ctx context.Context) (context.Context, func()) {
	// #nosec G118 -- the returned cleanup ends both parts of the request lifetime.
	req, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	if s.ctx.Err() != nil {
		cancel()
	}
	return req, func() { stop(); cancel() }
}

func (s *registryScope) cloudComputers(ctx context.Context) ([]remoteproto.AccountComputer, error) {
	var result []remoteproto.AccountComputer
	err := s.device(ctx, func(ctx context.Context, credentials cloudclient.Credentials) error {
		var err error
		result, err = s.registry.cloud.Computers(ctx, credentials)
		return err
	})
	return result, err
}

func (r *Registry) List(ctx context.Context) ([]View, error) {
	s, err := r.current()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := cmp.Or(ctx.Err(), s.ctx.Err()); err != nil {
		return nil, err
	}
	result := make([]View, 0, len(s.entries))
	for _, entry := range s.entries {
		result = append(result, entry.view)
	}
	slices.SortFunc(result, func(a, b View) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
	return result, nil
}

func (r *Registry) Get(ctx context.Context, id remoteproto.ComputerID) (View, error) {
	s, err := r.current()
	if err != nil {
		return View{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := cmp.Or(ctx.Err(), s.ctx.Err()); err != nil {
		return View{}, err
	}
	entry, ok := s.entries[id]
	if !ok {
		return View{}, ErrNotFound
	}
	return entry.view, nil
}

func (r *Registry) Available(ctx context.Context) ([]remoteproto.AccountComputer, error) {
	s, err := r.current()
	if err != nil {
		return nil, err
	}
	available, err := s.cloudComputers(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	return slices.DeleteFunc(available, func(item remoteproto.AccountComputer) bool {
		return item.AccountID != s.accountID || s.entries[item.ID] != nil
	}), nil
}

func initialView(computer store.Computer, connecting bool) View {
	status := Offline
	switch {
	case !computer.Enabled:
		status = Disabled
	case connecting:
		status = Connecting
	}
	return View{Computer: computer, Status: status, PlanState: PlanOK}
}

func (s *registryScope) refreshRoutes() {
	available, err := s.cloudComputers(s.ctx)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, info := range available {
		entry := s.entries[info.ID]
		if entry == nil || info.AccountID != s.accountID || s.ctx.Err() != nil {
			continue
		}
		s.refreshComputerRoutes(entry, info)
	}
}

func (s *registryScope) refreshComputerRoutes(entry *registryEntry, info remoteproto.AccountComputer) {
	store := s.registry.store
	routes, err := store.LearnComputerRoutes(s.ctx, s.accountID, entry.view.ID, advertisedRoutes(info))
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			s.registry.logger.WarnContext(s.ctx, "Could not refresh remote routes", "computer_id", entry.view.ID)
		}
		return
	}
	s.setRoutes(entry, routes)
}
