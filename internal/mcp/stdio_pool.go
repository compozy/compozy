package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// DefaultStdioIdleTimeout closes an unused stdio MCP child.
	DefaultStdioIdleTimeout = 10 * time.Minute
	// DefaultStdioConnectTimeout bounds one child launch plus MCP handshake.
	DefaultStdioConnectTimeout = 60 * time.Second
	// DefaultStdioMaxConcurrentCalls bounds in-flight requests on one stdio MCP child.
	DefaultStdioMaxConcurrentCalls = 8

	defaultStdioBackoffInitial = time.Second
	defaultStdioBackoffMax     = time.Minute
	// stdioRetireAfterTimeouts recycles a child that stopped answering.
	stdioRetireAfterTimeouts = 3
)

// StdioPoolConfig configures the daemon-owned stdio MCP session pool.
type StdioPoolConfig struct {
	// Root is a daemon-owned directory for isolated child homes and shared npm/uv package caches.
	// Empty keeps temporary homes under the OS temp directory with per-launch caches.
	Root               string
	IdleTimeout        time.Duration
	ConnectTimeout     time.Duration
	MaxConcurrentCalls int
	Logger             *slog.Logger

	backoffInitial time.Duration
	backoffMax     time.Duration
	now            func() time.Time
}

// StdioSessionPool keeps one live stdio MCP child per target and server launch identity so
// discovery, status probes, and tool calls reuse it instead of spawning a process per request.
type StdioSessionPool struct {
	cfg    StdioPoolConfig
	layout mcpStdioLayout

	processCtx    context.Context
	cancelProcess context.CancelFunc
	connectCtx    context.Context
	cancelConnect context.CancelFunc
	stopReaper    chan struct{}

	mu     sync.Mutex
	slots  map[string]*stdioSlot
	closed bool
	wg     sync.WaitGroup
}

// stdioSlot is the restart history of one target; it outlives the children it launches.
type stdioSlot struct {
	current      *stdioSession
	failures     int
	retryAt      time.Time
	lastErr      string
	failedDigest string
	lastStderr   *stdioStderrRing
	// secretsDigest/releaseSecrets keep the launch's resolved secrets registered with the
	// process-wide redactor while the slot retains its child or that child's diagnostics.
	secretsDigest      string
	releaseSecrets     func()
	releasePrevSecrets func()
	protocolDigest     string
	protocol           string
	lastSeen           time.Time
}

type stdioSession struct {
	slot    *stdioSlot
	digest  string
	stderr  *stdioStderrRing
	sem     chan struct{}
	ready   chan struct{}
	err     error
	session *mcpsdk.ClientSession

	// Guarded by StdioSessionPool.mu.
	inFlight   int
	lastUsed   time.Time
	startedAt  time.Time
	timeouts   int
	retiring   bool
	terminated bool
	closing    bool
}

// NewStdioSessionPool constructs a pool and starts its idle reaper. Close releases every child.
func NewStdioSessionPool(cfg StdioPoolConfig) (*StdioSessionPool, error) {
	cfg = normalizeStdioPoolConfig(cfg)
	layout, err := prepareMCPStdioLayout(cfg.Root)
	if err != nil {
		return nil, err
	}
	processCtx, cancelProcess := context.WithCancel(context.Background())
	connectCtx, cancelConnect := context.WithCancel(processCtx)
	pool := &StdioSessionPool{
		cfg:           cfg,
		layout:        layout,
		processCtx:    processCtx,
		cancelProcess: cancelProcess,
		connectCtx:    connectCtx,
		cancelConnect: cancelConnect,
		stopReaper:    make(chan struct{}),
		slots:         make(map[string]*stdioSlot),
	}
	pool.wg.Go(pool.reapIdle)
	return pool, nil
}

func normalizeStdioPoolConfig(cfg StdioPoolConfig) StdioPoolConfig {
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = DefaultStdioIdleTimeout
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = DefaultStdioConnectTimeout
	}
	if cfg.MaxConcurrentCalls <= 0 {
		cfg.MaxConcurrentCalls = DefaultStdioMaxConcurrentCalls
	}
	if cfg.backoffInitial <= 0 {
		cfg.backoffInitial = defaultStdioBackoffInitial
	}
	if cfg.backoffMax < cfg.backoffInitial {
		cfg.backoffMax = max(defaultStdioBackoffMax, cfg.backoffInitial)
	}
	if cfg.now == nil {
		cfg.now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	return cfg
}

func prepareMCPStdioLayout(root string) (mcpStdioLayout, error) {
	if root == "" {
		return mcpStdioLayout{}, nil
	}
	if !filepath.IsAbs(root) {
		return mcpStdioLayout{}, fmt.Errorf("mcp: stdio pool root %q must be absolute", root)
	}
	layout := mcpStdioLayout{
		homesDir: filepath.Join(root, "homes"),
		npmCache: filepath.Join(root, "npm"),
		uvCache:  filepath.Join(root, "uv"),
	}
	for _, dir := range []string{layout.homesDir, layout.npmCache, layout.uvCache} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return mcpStdioLayout{}, fmt.Errorf("mcp: create stdio pool directory %q: %w", dir, err)
		}
	}
	return layout, nil
}

type stdioLease struct {
	pool    *StdioSessionPool
	session *stdioSession
	once    sync.Once
}

// acquire returns a live session for spec, launching it at most once for concurrent callers.
// The launch runs on the pool lifetime, so a caller deadline never aborts a slow first start.
func (p *StdioSessionPool) acquire(ctx context.Context, spec mcpStdioSpec) (*stdioLease, error) {
	session, err := p.sessionFor(spec)
	if err != nil {
		return nil, err
	}
	select {
	case <-session.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if session.err != nil {
		return nil, session.err
	}
	select {
	case session.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	p.mu.Lock()
	if session.retiring || session.terminated {
		p.mu.Unlock()
		<-session.sem
		return nil, stdioUnavailableError("mcp stdio server session ended; retry the request", nil)
	}
	session.inFlight++
	session.lastUsed = p.cfg.now()
	p.mu.Unlock()
	return &stdioLease{pool: p, session: session}, nil
}

func (p *StdioSessionPool) sessionFor(spec mcpStdioSpec) (*stdioSession, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, stdioUnavailableError("mcp stdio session pool is closed", nil)
	}
	now := p.cfg.now()
	slot := p.slots[spec.slot]
	if slot == nil {
		slot = &stdioSlot{}
		p.slots[spec.slot] = slot
	}
	slot.lastSeen = now
	if current := slot.current; current != nil {
		if current.digest == spec.digest && !current.terminated && !current.retiring {
			return current, nil
		}
		p.retireLocked(current)
	}
	if slot.failedDigest != spec.digest {
		// A changed definition is a repair attempt; it does not inherit the old backoff.
		slot.failures = 0
		slot.retryAt = time.Time{}
	}
	if now.Before(slot.retryAt) {
		message := fmt.Sprintf(
			"mcp stdio server is backing off after a failure; next launch in %s",
			slot.retryAt.Sub(now).Round(time.Second),
		)
		return nil, stdioUnavailableError(message, errors.New(slot.lastErr))
	}
	slot.protectSecrets(spec)
	session := &stdioSession{
		slot:      slot,
		digest:    spec.digest,
		stderr:    newStdioStderrRing(defaultStdioStderrBufferBytes, spec.secrets),
		sem:       make(chan struct{}, p.cfg.MaxConcurrentCalls),
		ready:     make(chan struct{}),
		startedAt: now,
		lastUsed:  now,
	}
	slot.current = session
	p.wg.Go(func() { p.run(session, spec) })
	return session, nil
}

// knownProtocol reports the protocol negotiated by the last child launched for spec. A slot that is
// backing off after a failure reports none, so discovery surfaces the failure instead of stale tools.
func (p *StdioSessionPool) knownProtocol(spec mcpStdioSpec) (string, bool) {
	if p == nil {
		return "", false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	slot := p.slots[spec.slot]
	if p.closed || slot == nil || slot.protocolDigest != spec.digest || slot.protocol == "" {
		return "", false
	}
	if slot.current == nil && p.cfg.now().Before(slot.retryAt) {
		return "", false
	}
	return slot.protocol, true
}

// StderrTail returns the redacted stderr tail of the target's current or last stdio child.
func (p *StdioSessionPool) StderrTail(target mcpauth.Target) string {
	if p == nil {
		return ""
	}
	key, err := target.Normalize().Key()
	if err != nil {
		return ""
	}
	p.mu.Lock()
	slot := p.slots[key]
	var ring *stdioStderrRing
	if slot != nil {
		ring = slot.lastStderr
		if slot.current != nil {
			ring = slot.current.stderr
		}
	}
	p.mu.Unlock()
	return ring.Tail()
}

// protectSecrets registers spec's secrets for redaction until the slot is dropped. It keeps the
// previous definition's registration too, because a retired child may still be draining.
func (s *stdioSlot) protectSecrets(spec mcpStdioSpec) {
	if s.secretsDigest == spec.digest && s.releaseSecrets != nil {
		return
	}
	releases := make([]func(), 0, len(spec.secrets))
	for _, secret := range spec.secrets {
		releases = append(releases, diagnostics.RegisterRequiredSecret(secret))
	}
	if s.releasePrevSecrets != nil {
		s.releasePrevSecrets()
	}
	s.releasePrevSecrets = s.releaseSecrets
	s.secretsDigest = spec.digest
	s.releaseSecrets = func() {
		for _, release := range releases {
			release()
		}
	}
}

func (s *stdioSlot) releaseSecretProtection() {
	for _, release := range []func(){s.releasePrevSecrets, s.releaseSecrets} {
		if release != nil {
			release()
		}
	}
	s.releasePrevSecrets = nil
	s.releaseSecrets = nil
	s.secretsDigest = ""
}

func (l *stdioLease) release(callErr error) {
	if l == nil {
		return
	}
	l.once.Do(func() {
		l.pool.releaseSession(l.session, callErr)
	})
}

func (p *StdioSessionPool) releaseSession(session *stdioSession, callErr error) {
	p.mu.Lock()
	session.inFlight--
	session.lastUsed = p.cfg.now()
	switch {
	case callErr == nil:
		session.timeouts = 0
		session.slot.failures = 0
	case isMCPCallTimeout(callErr):
		session.timeouts++
		if session.timeouts >= stdioRetireAfterTimeouts {
			p.cfg.Logger.Warn("mcp: recycling unresponsive stdio server", "timeouts", session.timeouts)
			p.retireLocked(session)
		}
	}
	if session.retiring && session.inFlight == 0 {
		p.closeSessionLocked(session)
	}
	p.mu.Unlock()
	<-session.sem
}

func isMCPCallTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, toolspkg.ErrToolTimedOut) {
		return true
	}
	reason, ok := toolspkg.ReasonOf(err)
	return ok && reason == toolspkg.ReasonCallTimedOut
}

// retireLocked detaches session from its slot and closes it once no request uses it.
func (p *StdioSessionPool) retireLocked(session *stdioSession) {
	if session.slot.current == session {
		session.slot.current = nil
		session.slot.lastStderr = session.stderr
	}
	session.retiring = true
	if session.inFlight == 0 {
		p.closeSessionLocked(session)
	}
}

// closeSessionLocked starts an asynchronous graceful close of a connected session.
// A session still connecting is closed by run once its launch settles.
func (p *StdioSessionPool) closeSessionLocked(session *stdioSession) {
	if session.closing || session.terminated || session.session == nil {
		return
	}
	session.closing = true
	client := session.session
	p.wg.Go(func() {
		if err := client.Close(); err != nil {
			p.cfg.Logger.Debug("mcp: close retired stdio session", "error", redactMCPDiagnosticString(err.Error()))
		}
	})
}

// Close stops every child and waits for their homes to be removed. When ctx expires first,
// remaining children are killed through the process lifetime context.
func (p *StdioSessionPool) Close(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	close(p.stopReaper)
	p.cancelConnect()
	for _, slot := range p.slots {
		if slot.current != nil {
			p.retireLocked(slot.current)
		}
	}
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	defer p.releaseAllSecretProtection()
	var err error
	select {
	case <-done:
	case <-ctx.Done():
		err = fmt.Errorf("mcp: close stdio session pool: %w", ctx.Err())
		p.cancelProcess()
		<-done
	}
	p.cancelProcess()
	return err
}

func (p *StdioSessionPool) releaseAllSecretProtection() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, slot := range p.slots {
		slot.releaseSecretProtection()
	}
}

func stdioUnavailableError(message string, cause error) error {
	wrapped := toolspkg.ErrToolUnavailable
	if cause != nil && cause.Error() != "" {
		wrapped = fmt.Errorf("%w: %w", toolspkg.ErrToolUnavailable, cause)
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeUnavailable,
		"",
		message,
		wrapped,
		toolspkg.ReasonMCPUnreachable,
	)
}
