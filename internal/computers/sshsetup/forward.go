package sshsetup

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

type forwardKey struct {
	owner context.Context
	route string
}

type forwardProcess struct {
	cancel  context.CancelFunc
	changed chan struct{}
	address string
	err     error
}

type forwardPool struct {
	ssh       openSSH
	pins      string
	ask       func(context.Context, string) askpassPrompt
	mu        sync.Mutex
	processes map[forwardKey]*forwardProcess
	workers   sync.WaitGroup
	closed    bool
}

func (m *Manager) Forward(ctx, owner context.Context, route store.ComputerRoute) (string, error) {
	return m.forwards.address(ctx, owner, route)
}

func (p *forwardPool) address(ctx, owner context.Context, route store.ComputerRoute) (string, error) {
	if route.SSH == nil {
		return "", invalidSSHRequest()
	}
	t, err := targetFromAddress(route.SSH.Alias, route.Address)
	if err != nil {
		return "", err
	}
	listener, err := netip.ParseAddrPort(cmp.Or(route.SSH.TunnelAddress, config.DefaultRemoteListener))
	if err != nil || !listener.Addr().Unmap().IsLoopback() || listener.Port() == 0 {
		return "", invalidSSHRequest()
	}
	p.mu.Lock()
	if p.closed || owner.Err() != nil {
		p.mu.Unlock()
		return "", context.Canceled
	}
	key := forwardKey{owner, route.ID}
	process := p.processes[key]
	if process == nil {
		process = p.start(key, t, listener.String())
	}
	for {
		addr, result, changed := process.address, process.err, process.changed
		p.mu.Unlock()
		if addr != "" || result != nil {
			return addr, result
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-owner.Done():
			return "", owner.Err()
		case <-changed:
			p.mu.Lock()
		}
	}
}

func (p *forwardPool) start(key forwardKey, t target, destination string) *forwardProcess {
	// #nosec G118 -- the route owner or Close cancels the process worker.
	ctx, cancel := context.WithCancel(key.owner)
	process := &forwardProcess{cancel: cancel, changed: make(chan struct{})}
	p.processes[key] = process
	p.workers.Go(func() {
		defer cancel()
		defer func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			delete(p.processes, key)
		}()
		delay := time.Second
		for ctx.Err() == nil {
			ready, err := p.connect(ctx, process, t, destination)
			p.publish(process, "", cmp.Or(err, error(errInterrupted)))
			if ready {
				delay = time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
			case <-timer.C:
			}
			timer.Stop()
			delay = min(2*delay, 30*time.Second)
		}
	})
	return process
}

func (p *forwardPool) publish(process *forwardProcess, address string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	process.address, process.err = address, err
	close(process.changed)
	process.changed = make(chan struct{})
}

func (p *forwardPool) connect(
	ctx context.Context, process *forwardProcess, t target, destination string,
) (bool, error) {
	listener, err := new(net.ListenConfig).Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		return false, err
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return false, err
	}
	lifetime, cancel := context.WithCancel(ctx)
	defer cancel()
	env, stop, interactive, err := p.forwardAskpass(lifetime, t)
	if err != nil {
		return false, err
	}
	defer stop()
	marker := "compozy-forward-" + rand.Text()
	args := forwardArgs(t, p.pins, address, destination, interactive)
	args = append([]string{"-o", "PermitLocalCommand=yes", "-o", "LocalCommand=echo " + marker}, args...)
	forwardReady := make(chan struct{})
	signal := sync.OnceFunc(func() { close(forwardReady) })
	stdout := &sshOutput{onWrite: func(output []byte) {
		if bytes.Contains(output, []byte(marker)) {
			signal()
		}
	}}
	done := make(chan error, 1)
	go func() {
		result, err := runSSHCommand(p.ssh.command(lifetime, args, nil, env), stdout)
		done <- sshFailure(t, result, err)
	}()
	ready, exited, err := awaitForward(lifetime, forwardReady, done)
	if exited {
		return false, err
	}
	if ready {
		p.publish(process, address, nil)
	} else {
		cancel()
	}
	return ready, <-done
}

func (p *forwardPool) forwardAskpass(ctx context.Context, t target) ([]string, func(), bool, error) {
	ask := p.ask(ctx, t.address())
	if ask == nil {
		return nil, func() {}, false, nil
	}
	env, stop, err := serveAskpass(ctx, ask)
	return env, stop, true, err
}

func awaitForward(ctx context.Context, forwarded <-chan struct{}, done <-chan error) (ready, exited bool, err error) {
	startup := time.NewTimer(commandBudget)
	defer startup.Stop()
	select {
	case err := <-done:
		return false, true, err
	case <-ctx.Done():
		return false, false, ctx.Err()
	case <-startup.C:
		return false, false, errInterrupted
	case <-forwarded:
		// OpenSSH runs LocalCommand after binding its forwards with ExitOnForwardFailure.
		select {
		case err := <-done:
			return false, true, err
		default:
			return true, false, nil
		}
	}
}

func forwardArgs(t target, pins, local, remote string, interactive bool) []string {
	batch := "BatchMode=yes"
	if interactive {
		batch = "BatchMode=no"
	}
	return append(sshArgs(t, pins),
		"-o", "ExitOnForwardFailure=yes", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=3",
		"-o", "NumberOfPasswordPrompts=3", "-o", batch, "-N", "-L", local+":"+remote, "--", t.Alias)
}

func (p *forwardPool) close() {
	p.mu.Lock()
	p.closed = true
	for _, process := range p.processes {
		process.cancel()
	}
	p.mu.Unlock()
	p.workers.Wait()
}

func (m *Manager) Uninstall(ctx context.Context, route store.ComputerRoute) error {
	if route.SSH == nil {
		return invalidSSHRequest()
	}
	t, err := targetFromAddress(route.SSH.Alias, route.Address)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, commandBudget)
	defer cancel()
	command := "bin=$(" + findCompozy + "); test -n \"$bin\" || exit 0; " +
		"\"$bin\" service uninstall --json >/dev/null && " +
		"if test \"$bin\" = \"$HOME/.compozy/bin/compozy\"; then " +
		"rm -f \"$bin\" && rm -rf \"$HOME/.compozy/runtime\"; fi"
	args := commandArgs(t, m.pins, command)
	for i, arg := range args {
		if strings.HasPrefix(arg, "BatchMode=") {
			args[i] = "BatchMode=yes"
		}
	}
	result, err := m.ssh.run(ctx, args, nil, nil)
	return sshFailure(t, result, errors.Join(err, ctx.Err()))
}
