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
	"github.com/compozy/compozy/internal/store"
)

const (
	routeHTTPScheme     = "http"
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

func compareRoutes(a, b store.ComputerRoute) int {
	return cmp.Or(
		cmp.Compare(routePriority(a.Kind), routePriority(b.Kind)),
		cmp.Compare(a.Position, b.Position),
		cmp.Compare(a.ID, b.ID),
	)
}

func advertisedRoutes(info remoteproto.AccountComputer) []store.ComputerRoute {
	routes := make([]store.ComputerRoute, 0, len(info.Endpoints)+1)
	for _, address := range info.Endpoints {
		if socketHost(address) != "" && !slices.ContainsFunc(routes, func(route store.ComputerRoute) bool {
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

func learnedRoute(kind remoteproto.RouteKind, address string, position int) store.ComputerRoute {
	return store.ComputerRoute{ID: "rt_" + rand.Text(), Kind: kind, Address: address, Position: position, Learned: true}
}

func socketHost(address string) string {
	host, rawPort, err := net.SplitHostPort(address)
	if err != nil {
		return ""
	}
	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil || port == 0 {
		return ""
	}
	return host
}

func (r *Registry) routeOrigin(ctx context.Context, route store.ComputerRoute) (*url.URL, error) {
	switch route.Kind {
	case remoteproto.RouteDirect:
		if socketHost(route.Address) != "" {
			return &url.URL{Scheme: routeHTTPScheme, Host: route.Address}, nil
		}
	case remoteproto.RouteSSH:
		if r.forward != nil {
			socket, err := r.forward(ctx, route)
			if err != nil {
				return nil, ErrOffline
			}
			if ip := net.ParseIP(socketHost(socket)); ip != nil && ip.IsLoopback() {
				return &url.URL{Scheme: routeHTTPScheme, Host: socket}, nil
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
