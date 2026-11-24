package remotehost

import (
	"context"
	"errors"
	"maps"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/config"
)

const headerTimeout = 5 * time.Second

type listenAddress struct {
	kind    Listener
	address string
}

type listenerSet struct {
	mu      sync.Mutex
	entries map[listenAddress]*listenerRun
	closed  bool
}

type listenerRun struct {
	listener net.Listener
	server   *http.Server
	cancel   context.CancelFunc
	done     chan struct{}
}

func (s *listenerSet) apply(
	ctx context.Context, cfg config.RemoteConfig, handler func(Listener) http.Handler,
	publish func([]string) error,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrNotLinked
	}
	desired, err := listenerAddresses(cfg)
	if err != nil {
		return err
	}
	next, created, err := stageListeners(ctx, desired, s.entries)
	if err != nil {
		return err
	}
	if err := publish(boundAddresses(next, ListenerDirect)); err != nil {
		return errors.Join(err, closeStagedListeners(created))
	}
	for addr, run := range created {
		run.serve(ctx, handler(addr.kind))
	}
	for addr, run := range s.entries {
		if _, keep := next[addr]; !keep {
			run.close()
		}
	}
	s.entries = next
	return nil
}

func (s *listenerSet) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for _, run := range s.entries {
		run.close()
	}
	clear(s.entries)
}

func (s *listenerSet) addresses(kind Listener) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return boundAddresses(s.entries, kind)
}

func boundAddresses(entries map[listenAddress]*listenerRun, kind Listener) []string {
	addresses := make([]string, 0, len(entries))
	for addr, run := range entries {
		if addr.kind == kind {
			addresses = append(addresses, run.listener.Addr().String())
		}
	}
	slices.Sort(addresses)
	return addresses
}

func listenerAddresses(cfg config.RemoteConfig) ([]listenAddress, error) {
	addresses := []listenAddress{{ListenerTunnel, cfg.Listener}}
	if cfg.CloudRoute.Enabled {
		addresses = append(addresses, listenAddress{ListenerHosted, cfg.HostedListener})
	}
	if !cfg.Direct.Enabled {
		return addresses, nil
	}
	direct := cfg.Direct.Addresses
	if len(direct) == 0 {
		interfaces, err := net.InterfaceAddrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range interfaces {
			if network, ok := addr.(*net.IPNet); ok {
				direct = append(direct, network.IP.String())
			}
		}
	}
	for _, input := range direct {
		if addr, ok := cfg.DirectAddress(input); ok {
			item := listenAddress{ListenerDirect, addr}
			if !slices.Contains(addresses, item) {
				addresses = append(addresses, item)
			}
		}
	}
	return addresses, nil
}

func stageListeners(
	ctx context.Context, desired []listenAddress, existing map[listenAddress]*listenerRun,
) (map[listenAddress]*listenerRun, map[listenAddress]*listenerRun, error) {
	next := make(map[listenAddress]*listenerRun, len(desired))
	created := make(map[listenAddress]*listenerRun)
	var config net.ListenConfig
	for _, address := range desired {
		if run := existing[address]; run != nil {
			next[address] = run
			continue
		}
		socket, err := config.Listen(ctx, "tcp", address.address)
		if err != nil {
			return nil, nil, errors.Join(err, closeStagedListeners(created))
		}
		created[address] = &listenerRun{listener: socket}
	}
	maps.Copy(next, created)
	return next, created, nil
}

func closeStagedListeners(entries map[listenAddress]*listenerRun) error {
	var err error
	for _, run := range entries {
		err = errors.Join(err, run.listener.Close())
	}
	return err
}

func (r *listenerRun) serve(parent context.Context, handler http.Handler) {
	// #nosec G118 -- the listener owns cancel until close.
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	r.done = make(chan struct{})
	r.server = &http.Server{
		Handler: handler, ReadHeaderTimeout: headerTimeout, IdleTimeout: 30 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	go func() {
		defer close(r.done)
		_ = r.server.Serve(r.listener)
	}()
}

func (r *listenerRun) close() {
	r.cancel()
	_ = r.server.Close()
	<-r.done
}
