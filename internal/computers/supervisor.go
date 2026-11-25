package computers

import (
	"context"
	"errors"
	"maps"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
)

const (
	reconnectInterval = time.Second
	renewalLead       = 5 * time.Minute
	renewalRetry      = 5 * time.Second
)

type supervisor struct {
	scope    *registryScope
	ctx      context.Context
	cancel   context.CancelFunc
	computer store.Computer
	health   map[string]routeHealth
	errors   map[string]string
}

type routeProbe struct {
	route      store.ComputerRoute
	origin     *url.URL
	descriptor remoteproto.Descriptor
	problem    routeProblem
}

type routeProblem struct {
	step     string
	err      error
	protocol *remoteproto.ProtocolRange
}

func (s *registryScope) start(entry *registryEntry) {
	if !entry.view.Enabled || s.ctx.Err() != nil {
		return
	}
	// #nosec G118 -- the entry owns cancel until it is replaced or the account changes.
	ctx, cancel := context.WithCancel(s.ctx)
	routes := slices.Clone(entry.view.Routes)
	slices.SortFunc(routes, compareRoutes)
	entry.view.Routes = routes
	run := &supervisor{
		scope: s, ctx: ctx, cancel: cancel, computer: entry.view.Computer,
		health: make(map[string]routeHealth), errors: make(map[string]string),
	}
	entry.run = run
	s.workers.Go(run.run)
}

func (s *registryScope) stop(entry *registryEntry) {
	if entry.run != nil {
		entry.run.cancel()
		entry.run = nil
	}
	if entry.link != nil {
		entry.link.close()
		entry.link = nil
	}
}

func (s *registryScope) restart(entry *registryEntry) {
	s.stop(entry)
	entry.fault = nil
	entry.view = initialView(entry.view.Computer, len(entry.view.Routes) > 0)
	s.start(entry)
}

func (s *supervisor) run() {
	var link *connection
	defer func() {
		if link != nil {
			link.close()
		}
	}()
	for s.ctx.Err() == nil {
		if link == nil {
			probes := s.probe(s.computer.Routes)
			var problem routeProblem
			link, problem = s.connect(probes)
			if link == nil {
				s.report(problem)
				if !s.retry(problem.err) {
					return
				}
				continue
			}
			if !s.install(link, nil) {
				return
			}
		}
		replacement, failure := s.observe(link)
		if replacement != nil {
			if !s.install(replacement, link) {
				replacement.close()
				return
			}
		} else if s.ctx.Err() == nil {
			if errors.Is(failure, ErrOffline) {
				s.health[link.route.ID] = s.health[link.route.ID].dropped(s.scope.registry.clock.Now())
			}
			s.errors[link.route.ID] = describe(failure).Code
			s.report(activeFailure(failure))
			link.drainPolicyClose(s.ctx, failure)
		}
		link.close()
		link = replacement
	}
}

func (s *supervisor) retry(err error) bool {
	clock := s.scope.registry.clock
	delay := reconnectInterval
	if describe(err).Code == "device_pending_approval" {
		delay = 5 * time.Second
	}
	timer := clock.NewTimer(max(delay, cloudclient.RetryDelay(clock.Now(), err)))
	defer timer.Stop()
	select {
	case <-s.ctx.Done():
		return false
	case <-timer.Chan():
		return true
	}
}

func (s *supervisor) probe(routes []store.ComputerRoute) []routeProbe {
	r := s.scope.registry
	probes := make([]routeProbe, len(routes))
	var jobs sync.WaitGroup
	now := r.clock.Now()
	for i, route := range routes {
		probes[i].route = route
		if now.Before(s.health[route.ID].heldUntil) {
			continue
		}
		jobs.Go(func() { probes[i] = s.preflight(route) })
	}
	jobs.Wait()
	return probes
}

func (s *supervisor) preflight(route store.ComputerRoute) routeProbe {
	ctx, cancel := context.WithTimeout(s.ctx, probeTimeout)
	defer cancel()
	probe := routeProbe{route: route}
	r := s.scope.registry
	origin, err := r.routeOrigin(ctx, s.ctx, route)
	if err != nil {
		probe.problem = routeProblem{step: stepRoute, err: err}
		return probe
	}
	descriptor, err := fetchDescriptor(ctx, origin, r.routeHTTP(route.Kind))
	probe.descriptor = descriptor
	switch {
	case err != nil:
		probe.problem = routeProblem{step: stepRoute, err: err}
	case descriptor.ComputerID != s.computer.ID:
		probe.problem = routeProblem{step: stepRoute, err: ErrIdentityMismatch}
	case descriptor.TLSRootSHA256 == "" || descriptor.TLSRootSHA256 != s.computer.TLSRootSHA:
		probe.problem = routeProblem{step: stepRoute, err: ErrPinMismatch}
	default:
		if _, ok := remoteproto.CurrentProtocol.Overlap(descriptor.Protocol); ok {
			probe.origin = origin
		} else {
			probe.problem = routeProblem{step: stepComputer, err: ErrIncompatible, protocol: &descriptor.Protocol}
		}
	}
	return probe
}

func (s *supervisor) connect(probes []routeProbe) (*connection, routeProblem) {
	best := routeProblem{step: stepRoute, err: ErrOffline}
	for _, probe := range probes {
		if s.ctx.Err() != nil {
			break
		}
		problem := probe.problem
		if probe.origin != nil {
			link, failure := s.dial(probe)
			if link != nil {
				delete(s.errors, probe.route.ID)
				return link, routeProblem{}
			}
			problem = failure
		}
		if problem.err == nil {
			continue
		}
		s.errors[probe.route.ID] = describe(problem.err).Code
		if errors.Is(best.err, ErrOffline) && !errors.Is(problem.err, ErrOffline) {
			best = problem
		}
		if problem.step == stepCloud && cloudclient.RetryDelay(s.scope.registry.clock.Now(), problem.err) > 0 {
			return nil, problem
		}
	}
	return nil, best
}

func (s *supervisor) dial(probe routeProbe) (*connection, routeProblem) {
	r := s.scope.registry
	tunnel, err := dialTunnel(s.ctx, probe.origin, r.routeHTTP(probe.route.Kind), s.computer.TLSRootSHA, r.clock.Now())
	if err != nil {
		if !errors.Is(err, ErrPinMismatch) {
			err = ErrOffline
		}
		return nil, routeProblem{step: stepRoute, err: err}
	}
	link := &connection{tunnel: tunnel, route: probe.route, descriptor: probe.descriptor}
	if failure := s.mint(s.ctx, link); failure.err != nil {
		tunnel.close()
		return nil, failure
	}
	return link, routeProblem{}
}

func (s *supervisor) mint(ctx context.Context, link *connection) routeProblem {
	r := s.scope.registry
	var auth *connectionAuth
	var failure routeProblem
	err := s.scope.device(ctx, func(ctx context.Context, credentials cloudclient.Credentials) error {
		grant, err := r.cloud.ComputerGrant(ctx, credentials, s.computer.ID, link.route.Kind)
		if err != nil {
			return err
		}
		session, err := link.tunnel.mint(ctx, grant, credentials.Key, r.clock.Now())
		if err != nil {
			failure = activeFailure(err)
			return err
		}
		auth = &connectionAuth{
			key: credentials.Key, credential: session.Credential,
			expiresAt: time.Unix(session.ExpiresAt, 0), scopes: session.Scopes,
		}
		return nil
	})
	if err != nil {
		if failure.err != nil {
			return failure
		}
		return routeProblem{step: stepCloud, err: err}
	}
	link.auth.Store(auth)
	return routeProblem{}
}

func (s *supervisor) observe(link *connection) (*connection, error) {
	clock := s.scope.registry.clock
	health := clock.NewTicker(healthInterval)
	defer health.Stop()
	better := clock.NewTicker(betterRouteInterval)
	defer better.Stop()
	renewal := clock.NewTimer(renewalDelay(link.auth.Load(), clock.Now()))
	defer renewal.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return nil, s.ctx.Err()
		case <-link.mux.CloseChan():
			return nil, ErrOffline
		case <-health.Chan():
			if err := link.check(s.ctx, clock.Now()); err != nil {
				return nil, err
			}
		case <-renewal.Chan():
			if err := s.renew(link, renewal); err != nil {
				return nil, err
			}
		case <-better.Chan():
			if replacement := s.betterLink(link); replacement != nil {
				return replacement, nil
			}
		}
	}
}

func renewalDelay(auth *connectionAuth, now time.Time) time.Duration {
	return max(renewalRetry, auth.expiresAt.Add(-renewalLead).Sub(now))
}

func (s *supervisor) renew(link *connection, timer clockwork.Timer) error {
	clock := s.scope.registry.clock
	if !clock.Now().Before(link.auth.Load().expiresAt) {
		return ErrSessionExpired
	}
	ctx, cancel := context.WithTimeout(s.ctx, headerTimeout)
	defer cancel()
	failure := s.mint(ctx, link)
	now := clock.Now()
	if failure.err != nil {
		remaining := link.auth.Load().expiresAt.Sub(now)
		if remaining <= 0 {
			return failure.err
		}
		timer.Reset(min(remaining, max(renewalRetry, cloudclient.RetryDelay(now, failure.err))))
		return nil
	}
	auth := link.auth.Load()
	timer.Reset(renewalDelay(auth, now))
	s.scope.mu.Lock()
	defer s.scope.mu.Unlock()
	if entry := s.currentEntry(); entry != nil && entry.link == link && !slices.Equal(entry.view.Scopes, auth.scopes) {
		entry.view.Scopes = auth.scopes
		s.scope.changed(entry.view)
	}
	return nil
}

func (s *supervisor) betterLink(current *connection) *connection {
	index := slices.IndexFunc(s.computer.Routes, func(route store.ComputerRoute) bool {
		return route.ID == current.route.ID
	})
	if index <= 0 {
		return nil
	}
	probes := s.probe(s.computer.Routes[:index])
	replacement, _ := s.connect(probes)
	if replacement == nil {
		s.scope.mu.Lock()
		defer s.scope.mu.Unlock()
		entry := s.currentEntry()
		if entry != nil && !maps.Equal(entry.view.RouteErrors, s.errors) {
			entry.view.RouteErrors = maps.Clone(s.errors)
			s.scope.changed(entry.view)
		}
	}
	return replacement
}

func (s *supervisor) currentEntry() *registryEntry {
	entry := s.scope.entries[s.computer.ID]
	if entry == nil || entry.run != s || s.ctx.Err() != nil {
		return nil
	}
	return entry
}

func (s *supervisor) install(link, previous *connection) bool {
	scope := s.scope
	r := scope.registry
	scope.mu.Lock()
	defer scope.mu.Unlock()
	entry := s.currentEntry()
	if entry == nil {
		return false
	}
	at := r.clock.Now().UTC()
	if err := r.store.RecordComputerConnection(s.ctx, scope.accountID, link.descriptor, at); err != nil {
		r.logger.WarnContext(s.ctx, "Could not record remote computer connection", "computer_id", entry.view.ID)
	}
	if s.ctx.Err() != nil {
		return false
	}
	entry.link = link
	entry.fault = nil
	view := &entry.view
	view.Status, view.PlanState, view.Diagnosis = Connected, PlanOK, nil
	if warning := sshServiceWarning(view.Routes, at); warning != nil {
		view.Status, view.Diagnosis = NeedsAttention, warning
	}
	view.LastConnAt = &at
	view.Platform, view.Version = link.descriptor.Platform, link.descriptor.Version
	view.RouteInUse, view.Protocol = &link.route, &link.descriptor.Protocol
	view.Scopes, view.RouteErrors = link.auth.Load().scopes, maps.Clone(s.errors)
	scope.changed(*view)
	if previous != nil {
		r.logger.InfoContext(s.ctx, "computers.route_switch", "computer_id", view.ID,
			"from", previous.route.Kind, "to", link.route.Kind, "reason", "better_route")
	}
	return true
}

func (s *supervisor) report(problem routeProblem) {
	scope := s.scope
	scope.mu.Lock()
	defer scope.mu.Unlock()
	entry := s.currentEntry()
	if entry == nil {
		return
	}
	status, diagnosis := diagnose(problem, scope.registry.clock.Now().UTC())
	entry.fault = problem.err
	view := &entry.view
	if entry.link == nil && status == view.Status && sameDiagnosis(diagnosis, view.Diagnosis) &&
		maps.Equal(view.RouteErrors, s.errors) {
		return
	}
	entry.link = nil
	view.Status, view.Diagnosis = status, diagnosis
	view.RouteInUse, view.Scopes, view.Protocol = nil, nil, problem.protocol
	view.RouteErrors = maps.Clone(s.errors)
	if status == NeedsPro {
		view.PlanState = PlanLimited
	} else {
		view.PlanState = PlanOK
	}
	scope.changed(*view)
}
