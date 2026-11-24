// SPDX-License-Identifier: FSL-1.1-MIT

package cloud

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

const DrainTimeout = 60 * time.Second

func (a *App) Serve(ctx context.Context, listener net.Listener) error {
	lifetime, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()
	var workers sync.WaitGroup
	workers.Go(func() { a.RunBackground(lifetime) })
	defer func() {
		cancel()
		workers.Wait()
	}()

	server := &http.Server{
		Handler:           a,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
		BaseContext:       func(net.Listener) context.Context { return lifetime },
	}
	exited := make(chan error, 1)
	go func() { exited <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
		return a.shutdown(ctx, server)
	case err := <-exited:
		a.StartDrain()
		return err
	}
}

func (a *App) shutdown(ctx context.Context, server *http.Server) error {
	drained := a.StartDrain()
	deadline, cancel := context.WithTimeout(context.WithoutCancel(ctx), DrainTimeout)
	defer cancel()
	if err := server.Shutdown(deadline); err != nil {
		return errors.Join(err, server.Close())
	}
	select {
	case <-drained:
		return nil
	case <-deadline.Done():
		return deadline.Err()
	}
}
