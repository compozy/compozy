package computers

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/service"
	"github.com/compozy/compozy/internal/store"
)

var _ sshsetup.Registry = (*Registry)(nil)

func (r *Registry) SSHSetups() *sshsetup.Manager { return r.setups }

func (r *Registry) sshScope(owner string) (*registryScope, error) {
	scope, err := r.current()
	if err == nil && scope.accountID != owner {
		return nil, ErrNotFound
	}
	return scope, err
}

func (r *Registry) HasSSHRoute(ctx context.Context, owner, address string) (bool, error) {
	scope, err := r.sshScope(owner)
	if err != nil {
		return false, err
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	for _, entry := range scope.entries {
		if slices.ContainsFunc(entry.view.Routes, func(route store.ComputerRoute) bool {
			return route.Kind == remoteproto.RouteSSH && route.Address == address
		}) {
			return true, nil
		}
	}
	return false, ctx.Err()
}

func (r *Registry) SSHUpdateTarget(ctx context.Context, owner string, id remoteproto.ComputerID) (
	store.ComputerRoute, error,
) {
	scope, err := r.sshScope(owner)
	if err != nil {
		return store.ComputerRoute{}, err
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	entry, err := scope.entry(ctx, id)
	if err != nil {
		return store.ComputerRoute{}, err
	}
	for _, route := range entry.view.Routes {
		if route.Kind == remoteproto.RouteSSH && route.SSH != nil {
			return route, nil
		}
	}
	return store.ComputerRoute{}, sshsetup.ErrUpdateManual
}

func (r *Registry) RegisterSSHComputer(
	ctx context.Context, owner, name string, id remoteproto.ComputerID, route store.ComputerRoute,
) error {
	scope, err := r.sshScope(owner)
	if err != nil {
		return err
	}
	route.ID = "rt_" + rand.Text()
	if name != "" {
		items, err := scope.cloudComputers(ctx)
		if err != nil {
			return err
		}
		index := slices.IndexFunc(items, func(item remoteproto.AccountComputer) bool {
			return item.ID == id && item.AccountID == owner
		})
		if index < 0 {
			return ErrNotFound
		}
		item := items[index]
		_, err = scope.add(ctx, store.Computer{
			ID: id, AccountID: owner, Name: name, Enabled: true, Platform: item.Platform, Version: item.Version,
			TLSRootSHA: item.TLSRootSHA256, Weight: DefaultWeight, AddedAt: r.clock.Now().UTC(),
			Routes: append(advertisedRoutes(item), route),
		}, true)
		if err != nil {
			return err
		}
	}
	return scope.commitSSHRoute(ctx, id, route, name == "")
}

func (s *registryScope) commitSSHRoute(
	ctx context.Context, id remoteproto.ComputerID, route store.ComputerRoute, refresh bool,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := s.entry(ctx, id)
	if err != nil {
		return err
	}
	routes := slices.Clone(entry.view.Routes)
	index := slices.IndexFunc(routes, func(item store.ComputerRoute) bool {
		return item.Kind == remoteproto.RouteSSH && item.Address == route.Address
	})
	if index >= 0 {
		route.ID = routes[index].ID
		if !refresh && routes[index].SSH != nil && *routes[index].SSH == *route.SSH {
			return nil
		}
		routes[index] = route
	} else {
		routes = append(routes, route)
		slices.SortFunc(routes, compareRoutes)
	}
	if err := s.registry.store.AddComputerRoute(ctx, s.accountID, id, route); err != nil {
		return err
	}
	entry.view.Routes = routes
	s.restart(entry)
	s.changed(entry.view)
	return nil
}

func (r *Registry) WaitForSSHComputer(ctx context.Context, owner string, id remoteproto.ComputerID) error {
	scope, err := r.sshScope(owner)
	if err != nil {
		return err
	}
	sub, err := r.Subscribe(ctx, "")
	if err != nil {
		return err
	}
	defer sub.Close()
	for {
		scope.mu.Lock()
		entry, err := scope.entry(ctx, id)
		if err != nil {
			scope.mu.Unlock()
			return err
		}
		ready, fault := entry.link != nil, entry.fault
		scope.mu.Unlock()
		if ready {
			return nil
		}
		if fault != nil && (errors.Is(fault, ErrIncompatible) || describe(fault).Code == "ssh_host_key_changed") {
			return fault
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-sub.Done:
			return ErrNotFound
		case _, ok := <-sub.Events:
			if !ok {
				return ErrOffline
			}
		}
	}
}

func (r *Registry) PublishSSHSetup(owner string, generation int64, status sshsetup.Status) {
	lifetime, identity := r.account.IdentityLifetime()
	if lifetime.Err() != nil || identity.AccountID != owner || identity.AuthGeneration != generation {
		return
	}
	scope, err := r.activate(lifetime, identity)
	if err != nil {
		return
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.ctx.Err() == nil {
		scope.events.publish(Change{Setup: &status})
	}
}

func (r *Registry) sshRouteOrigin(ctx, owner context.Context, route store.ComputerRoute) (*url.URL, error) {
	var socket string
	var err error
	switch {
	case r.forward != nil:
		socket, err = r.forward(ctx, route)
	case r.setups != nil:
		socket, err = r.setups.Forward(ctx, owner, route)
	}
	if fault, ok := errors.AsType[*sshsetup.Failure](err); ok && fault.Code == "ssh_host_key_changed" {
		return nil, &Failure{Status: fault.HTTPStatus, Code: fault.Code, Message: fault.Message}
	}
	if err == nil {
		if ip := net.ParseIP(socketHost(socket)); ip != nil && ip.IsLoopback() {
			return &url.URL{Scheme: routeHTTPScheme, Host: socket}, nil
		}
	}
	return nil, ErrOffline
}

func sshServiceWarning(routes []store.ComputerRoute, at time.Time) *Diagnosis {
	for _, route := range routes {
		if route.SSH != nil && route.SSH.ServiceWarning == service.LingerOff {
			return &Diagnosis{Step: sshsetup.StepService, Code: service.LingerOff, At: at,
				Message: fmt.Sprintf("Runs only while you're logged in on %s. Run sudo loginctl enable-linger %s",
					route.SSH.Alias, route.SSH.User)}
		}
	}
	return nil
}
