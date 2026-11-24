package computers

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
)

const (
	UninstallDone    = "done"
	UninstallFailed  = "failed"
	UninstallSkipped = "skipped"
)

func (r *Registry) AddFromAccount(ctx context.Context, id remoteproto.ComputerID) (View, error) {
	s, err := r.current()
	if err != nil {
		return View{}, err
	}
	s.mu.Lock()
	if entry, exists := s.entries[id]; exists && s.ctx.Err() == nil {
		view := entry.view
		s.mu.Unlock()
		return view, nil
	}
	s.mu.Unlock()
	available, err := s.cloudComputers(ctx)
	if err != nil {
		return View{}, err
	}
	index := slices.IndexFunc(available, func(info remoteproto.AccountComputer) bool {
		return info.ID == id && info.AccountID == s.accountID
	})
	if index < 0 {
		return View{}, ErrNotFound
	}
	info := available[index]
	routes := advertisedRoutes(info)
	_, identity := r.account.IdentityLifetime()
	if info.PlanLimited || (identity.Profile.Plan == "free" &&
		len(routes) == 1 && routes[0].Kind == remoteproto.RouteCloud) {
		return View{}, ErrPlanRequired
	}
	computer := store.Computer{
		ID: id, AccountID: s.accountID, Name: info.Name, Enabled: true,
		Platform: info.Platform, Version: info.Version, TLSRootSHA: info.TLSRootSHA256,
		Weight: DefaultWeight, Routes: routes, AddedAt: r.clock.Now().UTC(),
	}
	return s.add(ctx, computer, info.Online && len(routes) > 0)
}

func (s *registryScope) add(ctx context.Context, computer store.Computer, connecting bool) (View, error) {
	ctx, done := s.operation(ctx)
	defer done()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return View{}, err
	}
	if existing := s.entries[computer.ID]; existing != nil {
		return existing.view, nil
	}
	if err := s.registry.store.AddComputer(ctx, computer); err != nil {
		return View{}, err
	}
	entry := &registryEntry{view: initialView(computer, connecting)}
	s.entries[computer.ID] = entry
	s.start(entry)
	s.changed(entry.view)
	return entry.view, nil
}

func (r *Registry) Update(ctx context.Context, id remoteproto.ComputerID, patch store.ComputerPatch) (View, error) {
	if err := validatePatch(&patch); err != nil {
		return View{}, err
	}
	s, err := r.current()
	if err != nil {
		return View{}, err
	}
	ctx, done := s.operation(ctx)
	defer done()
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := s.entry(ctx, id)
	if err != nil {
		return View{}, err
	}
	if err := r.store.UpdateComputer(ctx, s.accountID, id, patch); err != nil {
		return View{}, computerStoreFailure(err)
	}
	if patch.Name != nil {
		entry.view.Name = *patch.Name
	}
	if patch.Weight != nil {
		entry.view.Weight = *patch.Weight
	}
	if patch.Enabled != nil && *patch.Enabled != entry.view.Enabled {
		entry.view.Enabled = *patch.Enabled
		s.restart(entry)
	}
	s.changed(entry.view)
	return entry.view, nil
}

func validatePatch(patch *store.ComputerPatch) error {
	if patch.Name != nil {
		patch.Name = new(strings.TrimSpace(*patch.Name))
		if *patch.Name == "" || *patch.Name == "local" {
			return invalidComputerRequest("Enter a name for the computer.")
		}
	}
	if patch.Weight != nil && !config.IsComputerWeight(*patch.Weight) {
		return invalidComputerRequest("Choose a load-balancing weight of 0, 25, 50, or 100.")
	}
	return nil
}

func invalidComputerRequest(message string) error {
	return &Failure{Status: http.StatusBadRequest, Code: "invalid_computer_request", Message: message}
}

func (r *Registry) Remove(ctx context.Context, id remoteproto.ComputerID, options RemoveOptions) (string, error) {
	s, err := r.current()
	if err != nil {
		return "", err
	}
	ctx, done := s.operation(ctx)
	defer done()
	s.mu.Lock()
	entry, err := s.entry(ctx, id)
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	routes := entry.view.Routes
	s.mu.Unlock()
	if options.Unlink {
		err := s.device(ctx, func(ctx context.Context, credentials cloudclient.Credentials) error {
			return r.cloud.UnlinkComputer(ctx, id, credentials)
		})
		if err != nil && cloudclient.FailureCode(err) != "computer_not_owned" {
			return "", err
		}
	}
	if err := s.remove(ctx, id); err != nil {
		return "", err
	}
	return r.uninstallRoute(ctx, routes, options.Uninstall), nil
}

func (s *registryScope) remove(ctx context.Context, id remoteproto.ComputerID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := s.entry(ctx, id)
	if err != nil {
		return err
	}
	if err := s.registry.store.RemoveComputer(ctx, s.accountID, id); err != nil {
		return computerStoreFailure(err)
	}
	s.stop(entry)
	delete(s.entries, id)
	s.events.publish(Change{ComputerID: id, Removed: true})
	return nil
}

func (r *Registry) uninstallRoute(ctx context.Context, routes []store.ComputerRoute, requested bool) string {
	if !requested {
		return UninstallSkipped
	}
	index := slices.IndexFunc(
		routes,
		func(route store.ComputerRoute) bool { return route.Kind == remoteproto.RouteSSH },
	)
	if index < 0 {
		return UninstallSkipped
	}
	if r.uninstall == nil || r.uninstall(ctx, routes[index]) != nil {
		return UninstallFailed
	}
	return UninstallDone
}

func (r *Registry) RemoveRoute(ctx context.Context, id remoteproto.ComputerID, routeID string) error {
	s, err := r.current()
	if err != nil {
		return err
	}
	ctx, done := s.operation(ctx)
	defer done()
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := s.entry(ctx, id)
	if err != nil {
		return err
	}
	if err := r.store.ForgetComputerRoute(ctx, s.accountID, id, routeID); err != nil {
		return computerStoreFailure(err)
	}
	s.setRoutes(entry, slices.DeleteFunc(slices.Clone(entry.view.Routes), func(route store.ComputerRoute) bool {
		return route.ID == routeID
	}))
	return nil
}

func computerStoreFailure(err error) error {
	switch {
	case errors.Is(err, store.ErrComputerNotFound):
		return ErrNotFound
	case errors.Is(err, store.ErrComputerRouteNotFound):
		return ErrRouteNotFound
	default:
		return err
	}
}

// AddSSHRoute saves an established forward's SSH target in the current account's registry.
func (r *Registry) AddSSHRoute(ctx context.Context, id remoteproto.ComputerID, route store.ComputerRoute) error {
	if route.Kind != remoteproto.RouteSSH || route.SSH == nil || route.Address == "" || route.Learned {
		return invalidComputerRequest("An SSH target is required.")
	}
	s, err := r.current()
	if err != nil {
		return err
	}
	ctx, done := s.operation(ctx)
	defer done()
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := s.entry(ctx, id)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(entry.view.Routes, func(existing store.ComputerRoute) bool {
		return existing.Kind == route.Kind && existing.Address == route.Address
	}) {
		return nil
	}
	route.ID = "rt_" + rand.Text()
	route.SSH = new(*route.SSH)
	if err := r.store.AddComputerRoute(ctx, s.accountID, id, route); err != nil {
		return err
	}
	s.setRoutes(entry, append(slices.Clone(entry.view.Routes), route))
	return nil
}

func (s *registryScope) entry(ctx context.Context, id remoteproto.ComputerID) (*registryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	entry := s.entries[id]
	if entry == nil {
		return nil, ErrNotFound
	}
	return entry, nil
}

func (s *registryScope) setRoutes(entry *registryEntry, routes []store.ComputerRoute) {
	slices.SortFunc(routes, compareRoutes)
	if slices.EqualFunc(entry.view.Routes, routes, func(a, b store.ComputerRoute) bool {
		return a.ID == b.ID && a.Kind == b.Kind && a.Address == b.Address && a.Position == b.Position
	}) {
		return
	}
	entry.view.Routes = routes
	s.restart(entry)
	s.changed(entry.view)
}
