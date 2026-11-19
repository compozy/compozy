package mcp

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpCatalogHTTP struct {
	mu         sync.Mutex
	workers    sync.WaitGroup
	sessions   map[string]mcpCatalogHTTPEntry
	changes    chan struct{}
	closed     bool
	invalidate func(string)
	expire     func(string)
}

type mcpCatalogHTTPEntry struct {
	session *mcpsdk.ClientSession
	stream  *catalogNotificationStream
	usedAt  time.Time
}

func (e *CallExecutor) openToolListingClient(
	ctx context.Context, resolved *ResolvedServer, authorizationHeader string,
) (*managedMCPClient, error) {
	catalog := e.catalogHTTP
	if catalog == nil || resolved.Server.EffectiveTransport() != compozyconfig.MCPServerTransportHTTP {
		return e.openRenderedClient(ctx, resolved, authorizationHeader)
	}
	key, err := e.toolCacheKey(ctx, resolved, "", authorizationHeader)
	if err != nil {
		return nil, err
	}
	if active := catalog.load(key); active.session != nil {
		return active.client(), nil
	}
	httpClient, err := e.httpClientWithAuthorization(ctx, resolved, authorizationHeader)
	if err != nil {
		return nil, err
	}
	// RPC deadlines come from the caller; bounded dial and response-header timeouts are
	// retained. The SDK owns the notification stream until the session is closed.
	httpClient.Timeout = 0
	stream := &catalogNotificationStream{next: httpClient.Transport, ended: func() { catalog.expire(key) }}
	stream.established = func() { e.markToolListingSubscribed(key, stream.live) }
	httpClient.Transport = stream
	session, err := connectMCPClientWithCatalogChanges(ctx, &mcpsdk.StreamableClientTransport{
		Endpoint: strings.TrimSpace(resolved.Server.URL), HTTPClient: httpClient, MaxRetries: -1,
	}, func(context.Context, *mcpsdk.ToolListChangedRequest) {
		catalog.notify(key)
	}, stream.observeAcknowledgement)
	if err != nil {
		return nil, err
	}
	if !supportsToolListChanges(session) {
		return &managedMCPClient{session: session}, nil
	}
	kept, retired := catalog.store(key, mcpCatalogHTTPEntry{
		session: session, stream: stream, usedAt: time.Now(),
	})
	retireErr := closeMCPClient(retired)
	if kept.session == nil {
		return nil, errors.Join(mcpsdk.ErrConnectionClosed, retireErr)
	}
	if retireErr != nil {
		slog.DebugContext(ctx, "mcp: close superseded HTTP catalog subscription failed")
	}
	return kept.client(), nil
}

func (e mcpCatalogHTTPEntry) client() *managedMCPClient {
	return &managedMCPClient{session: e.session, borrowed: true, catalogLive: e.stream.live}
}

func supportsToolListChanges(session *mcpsdk.ClientSession) bool {
	result := session.InitializeResult()
	return result != nil && result.Capabilities != nil && result.Capabilities.Tools != nil &&
		result.Capabilities.Tools.ListChanged
}

func (c *mcpCatalogHTTP) notify(key string) {
	c.invalidate(key)
	select {
	case c.changes <- struct{}{}:
	default:
	}
}

func (c *mcpCatalogHTTP) load(key string) mcpCatalogHTTPEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, found := c.sessions[key]
	if !found {
		return mcpCatalogHTTPEntry{}
	}
	entry.usedAt = time.Now()
	c.sessions[key] = entry
	return entry
}

func (c *mcpCatalogHTTP) store(
	key string, entry mcpCatalogHTTPEntry,
) (kept mcpCatalogHTTPEntry, retired *managedMCPClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return mcpCatalogHTTPEntry{}, &managedMCPClient{session: entry.session}
	}
	if current, found := c.sessions[key]; found {
		return current, &managedMCPClient{session: entry.session}
	}
	if len(c.sessions) >= defaultToolNormalizationCacheCapacity {
		var oldest string
		usedAt := entry.usedAt
		for candidate, previous := range c.sessions {
			if oldest == "" || previous.usedAt.Before(usedAt) {
				oldest, usedAt = candidate, previous.usedAt
			}
		}
		retired = &managedMCPClient{session: c.sessions[oldest].session}
		delete(c.sessions, oldest)
		c.expire(oldest)
	}
	c.sessions[key] = entry
	c.workers.Go(func() {
		if err := entry.session.Wait(); err != nil && !errors.Is(err, mcpsdk.ErrConnectionClosed) {
			slog.Debug("mcp: HTTP catalog connection ended")
		}
		if err := c.forget(entry.session); err != nil {
			slog.Debug("mcp: close expired HTTP catalog connection failed")
		}
	})
	return entry, retired
}

func (c *mcpCatalogHTTP) forget(session *mcpsdk.ClientSession) error {
	return c.retire(func(_ string, entry mcpCatalogHTTPEntry) bool { return entry.session == session }, false)
}

func (c *mcpCatalogHTTP) retire(match func(string, mcpCatalogHTTPEntry) bool, closeAll bool) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	c.closed = c.closed || closeAll
	var retired []*mcpsdk.ClientSession
	for key, entry := range c.sessions {
		if closeAll || match(key, entry) {
			retired = append(retired, entry.session)
			delete(c.sessions, key)
			c.expire(key)
		}
	}
	c.mu.Unlock()
	var err error
	for _, session := range retired {
		err = errors.Join(err, session.Close())
	}
	return err
}

// CloseCatalogConnections retires every HTTP discovery subscription at daemon shutdown.
func (e *CallExecutor) CloseCatalogConnections() error {
	if e.catalogHTTP == nil {
		return nil
	}
	if e.stdioPool != nil {
		e.stdioPool.retainCatalog(nil)
	}
	err := e.catalogHTTP.retire(nil, true)
	e.catalogHTTP.workers.Wait()
	return err
}

// CatalogChanges signals that at least one HTTP subscription invalidated its listing.
func (e *CallExecutor) CatalogChanges() <-chan struct{} {
	if e.catalogHTTP == nil {
		return nil
	}
	return e.catalogHTTP.changes
}

// RetainCatalogSubscriptions closes notification streams outside the active consented scopes.
func (e *CallExecutor) RetainCatalogSubscriptions(ctx context.Context, sources map[toolspkg.SourceRef]struct{}) error {
	if e.catalogHTTP == nil {
		return nil
	}
	active := make(map[string]bool, len(sources))
	polls := make(map[string]bool, len(sources))
	stdio := make(map[stdioCatalogBinding]struct{}, len(sources))
	for source := range sources {
		resolved, err := e.resolveServer(ctx, source, "")
		if err != nil {
			continue
		}
		if e.authorizeServer(ctx, source, &resolved) != nil {
			continue
		}
		header, err := e.authorizationHeader(ctx, &resolved)
		if err != nil {
			continue
		}
		if err := e.prepareToolListing(ctx, &resolved); err != nil {
			continue
		}
		key, err := e.toolPollKey(ctx, &resolved, header)
		if err != nil {
			continue
		}
		polls[key] = true
		if spec := resolved.listingSpec; spec != nil {
			stdio[stdioCatalogBinding{spec.slot, spec.digest}] = struct{}{}
			continue
		}
		active[key] = true
	}
	if e.stdioPool != nil {
		e.stdioPool.retainCatalog(stdio)
	}
	err := e.catalogHTTP.retire(func(key string, _ mcpCatalogHTTPEntry) bool {
		return !active[key]
	}, false)
	e.retainCatalogPolls(polls)
	return err
}
