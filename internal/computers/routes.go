package computers

import (
	"cmp"
	"context"
	"crypto/rand"
	"net"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	probeTimeout        = 2500 * time.Millisecond
	betterRouteInterval = time.Minute
	dropWindow          = 10 * time.Minute
	holdBack            = 5 * time.Minute
)

type routeHealth struct {
	lastDrop  time.Time
	heldUntil time.Time
}

func (h routeHealth) dropped(now time.Time) routeHealth {
	if !h.lastDrop.IsZero() && now.Sub(h.lastDrop) <= dropWindow {
		return routeHealth{heldUntil: now.Add(holdBack)}
	}
	return routeHealth{lastDrop: now}
}

func routePriority(kind remoteproto.RouteKind) int {
	switch kind {
	case remoteproto.RouteDirect:
		return 0
	case remoteproto.RouteSSH:
		return 1
	case remoteproto.RouteCloud:
		return 2
	default:
		return 3
	}
}

func compareRoutes(a, b Route) int {
	return cmp.Or(
		cmp.Compare(routePriority(a.Kind), routePriority(b.Kind)),
		cmp.Compare(a.Position, b.Position),
		cmp.Compare(a.ID, b.ID),
	)
}

func advertisedRoutes(info remoteproto.AccountComputer) []Route {
	routes := make([]Route, 0, len(info.Endpoints)+1)
	for _, address := range info.Endpoints {
		if validSocketAddress(address) && !slices.ContainsFunc(routes, func(route Route) bool {
			return route.Address == address
		}) {
			routes = append(routes, learnedRoute(remoteproto.RouteDirect, address, len(routes)))
		}
	}
	if info.HostedHostname != "" {
		routes = append(routes, learnedRoute(remoteproto.RouteCloud, info.HostedHostname, 0))
	}
	return routes
}

func learnedRoute(kind remoteproto.RouteKind, address string, position int) Route {
	return Route{ID: "rt_" + rand.Text(), Kind: kind, Address: address, Position: position, Learned: true}
}

func validSocketAddress(address string) bool {
	host, rawPort, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return false
	}
	port, err := strconv.ParseUint(rawPort, 10, 16)
	return err == nil && port > 0
}

func (r *Registry) routeOrigin(ctx context.Context, route Route) (*url.URL, error) {
	switch route.Kind {
	case remoteproto.RouteDirect:
		if validSocketAddress(route.Address) {
			return &url.URL{Scheme: "http", Host: route.Address}, nil
		}
	case remoteproto.RouteSSH:
		if r.forward != nil {
			socket, err := r.forward(ctx, route)
			if err != nil {
				return nil, ErrOffline
			}
			host, _, _ := net.SplitHostPort(socket)
			if ip := net.ParseIP(host); validSocketAddress(socket) && ip != nil && ip.IsLoopback() {
				return &url.URL{Scheme: "http", Host: socket}, nil
			}
		}
	case remoteproto.RouteCloud:
		target, err := url.Parse("https://" + route.Address)
		if err == nil && target.Hostname() != "" && target.Host == route.Address && target.User == nil {
			return target, nil
		}
	}
	return nil, ErrOffline
}
