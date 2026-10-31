package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// stdioStableUptime is how long a child must live before its exit stops escalating backoff.
	stdioStableUptime = time.Minute
	// legacyStdioHomeMinAge protects homes of per-call launches from older daemons still running.
	legacyStdioHomeMinAge = time.Hour
	maxStdioReapInterval  = 30 * time.Second
	minStdioReapInterval  = 10 * time.Millisecond
	// minStdioSlotRetention keeps an unused target's negotiated protocol (for cache-first discovery),
	// backoff history, and last stderr well past the idle close of its child.
	minStdioSlotRetention    = time.Hour
	stdioSlotRetentionFactor = 6
)

// run owns one child from launch to removal of its isolated home.
func (p *StdioSessionPool) run(session *stdioSession, spec mcpStdioSpec) {
	launch, err := newMCPStdioProcess(p.processCtx, spec, p.layout, session.stderr)
	if err != nil {
		p.finishConnect(session, nil, err)
		return
	}
	connectCtx, cancel := context.WithTimeout(p.connectCtx, p.cfg.ConnectTimeout)
	client, err := connectMCPClient(connectCtx, &mcpsdk.CommandTransport{Command: launch.command})
	cancel()
	if err != nil {
		// The SDK closes a session whose handshake fails; kill covers a child it never reached.
		if launch.command.Process != nil {
			if killErr := launch.command.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				err = errors.Join(err, fmt.Errorf("mcp: kill stdio server after failed connect: %w", killErr))
			}
		}
		if cleanupErr := launch.cleanup(); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("mcp: clean up stdio home: %w", cleanupErr))
		}
		p.finishConnect(session, nil, err)
		return
	}
	p.finishConnect(session, client, nil)
	waitErr := client.Wait()
	closeErr := client.Close()
	cleanupErr := launch.cleanup()
	p.finishSession(session, errors.Join(waitErr, closeErr), cleanupErr)
}

func (p *StdioSessionPool) finishConnect(session *stdioSession, client *mcpsdk.ClientSession, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	slot := session.slot
	if err != nil {
		session.err = withStdioStderr(err, session.stderr)
		session.terminated = true
		if slot.current == session {
			slot.current = nil
		}
		slot.lastStderr = session.stderr
		if !session.retiring {
			p.recordFailureLocked(slot, session.digest, session.err)
		}
		close(session.ready)
		return
	}
	session.session = client
	session.lastUsed = p.cfg.now()
	slot.protocolDigest = session.digest
	slot.protocol = negotiatedProtocolVersion(client)
	close(session.ready)
	if session.retiring && session.inFlight == 0 {
		p.closeSessionLocked(session)
	}
}

func (p *StdioSessionPool) finishSession(session *stdioSession, exitErr error, cleanupErr error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	slot := session.slot
	session.terminated = true
	if slot.current == session {
		slot.current = nil
	}
	slot.lastStderr = session.stderr
	if cleanupErr != nil {
		p.cfg.Logger.Warn("mcp: remove stdio server home", "error", redactMCPDiagnosticString(cleanupErr.Error()))
	}
	if session.retiring {
		return
	}
	if p.cfg.now().Sub(session.startedAt) >= stdioStableUptime {
		slot.failures = 0
	}
	cause := errors.New("mcp stdio server exited unexpectedly")
	if exitErr != nil {
		cause = fmt.Errorf("mcp stdio server exited unexpectedly: %w", exitErr)
	}
	p.recordFailureLocked(slot, session.digest, withStdioStderr(cause, session.stderr))
}

func (p *StdioSessionPool) recordFailureLocked(slot *stdioSlot, digest string, err error) {
	slot.failures++
	slot.failedDigest = digest
	delay := p.cfg.backoffInitial << min(slot.failures-1, 16)
	if delay <= 0 || delay > p.cfg.backoffMax {
		delay = p.cfg.backoffMax
	}
	slot.retryAt = p.cfg.now().Add(delay)
	slot.lastErr = redactMCPDiagnosticString(err.Error())
	p.cfg.Logger.Warn(
		"mcp: stdio server failed; backing off",
		"failures", slot.failures,
		"retry_in", delay.String(),
		"error", slot.lastErr,
	)
}

// withStdioStderr annotates a launch failure with the child's redacted stderr tail.
func withStdioStderr(err error, stderr *stdioStderrRing) error {
	tail := stderr.Tail()
	if tail == "" {
		return err
	}
	return fmt.Errorf("%w; stderr: %s", err, tail)
}

func (p *StdioSessionPool) reapIdle() {
	interval := min(max(p.cfg.IdleTimeout/4, minStdioReapInterval), maxStdioReapInterval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopReaper:
			return
		case <-ticker.C:
			p.reapIdleOnce()
		}
	}
}

func (p *StdioSessionPool) reapIdleOnce() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	now := p.cfg.now()
	retention := max(stdioSlotRetentionFactor*p.cfg.IdleTimeout, minStdioSlotRetention)
	for key, slot := range p.slots {
		current := slot.current
		if current != nil && current.session != nil && current.inFlight == 0 &&
			now.Sub(current.lastUsed) >= p.cfg.IdleTimeout {
			p.retireLocked(current)
		}
		if slot.current == nil && !now.Before(slot.retryAt) && now.Sub(slot.lastSeen) >= retention {
			slot.releaseSecretProtection()
			delete(p.slots, key)
		}
	}
}

// SweepOrphanHomes removes stdio homes left behind by an earlier daemon generation. Every home
// under the pool root belongs to this daemon home, whose lock the caller holds; legacy homes in the
// OS temp directory are removed only once they are old enough to rule out a live per-call launch.
func (p *StdioSessionPool) SweepOrphanHomes() (int, error) {
	if p == nil {
		return 0, nil
	}
	p.mu.Lock()
	active := len(p.slots)
	p.mu.Unlock()
	if active > 0 {
		return 0, errors.New("mcp: sweep stdio homes after sessions started")
	}
	var errs []error
	removed := 0
	if p.layout.homesDir != "" {
		count, err := removeStdioHomes(p.layout.homesDir, 0, p.cfg.now())
		removed += count
		errs = append(errs, err)
	}
	count, err := removeStdioHomes(os.TempDir(), legacyStdioHomeMinAge, p.cfg.now())
	removed += count
	errs = append(errs, err)
	return removed, errors.Join(errs...)
}

func removeStdioHomes(dir string, minAge time.Duration, now time.Time) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("mcp: list stdio homes in %q: %w", dir, err)
	}
	var errs []error
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), mcpStdioHomeNamePrefix) {
			continue
		}
		if minAge > 0 {
			info, err := entry.Info()
			if err != nil || now.Sub(info.ModTime()) < minAge {
				continue
			}
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			errs = append(errs, fmt.Errorf("mcp: remove orphaned stdio home %q: %w", entry.Name(), err))
			continue
		}
		removed++
	}
	return removed, errors.Join(errs...)
}
