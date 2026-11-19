package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpCatalogHTTP struct {
	mu       sync.Mutex
	sessions map[string]mcpCatalogHTTPEntry
	changes  chan struct{}
	closed   bool
}

type mcpCatalogHTTPEntry struct {
	key     string
	source  toolspkg.SourceRef
	session *mcpsdk.ClientSession
	usedAt  time.Time
}

func (e *CallExecutor) openToolListingClient(
	ctx context.Context, source toolspkg.SourceRef, resolved *ResolvedServer, authorizationHeader string,
) (*managedMCPClient, error) {
	catalog := e.catalogHTTP
	if catalog == nil || resolved.Server.EffectiveTransport() != compozyconfig.MCPServerTransportHTTP {
		return e.openRenderedClient(ctx, resolved, authorizationHeader)
	}
	slot, err := resolved.Target.Key()
	if err != nil {
		return nil, fmt.Errorf("mcp: identify HTTP catalog subscription: %w", err)
	}
	key, err := e.toolCacheKey(ctx, resolved, "", authorizationHeader)
	if err != nil {
		return nil, err
	}
	if session := catalog.load(slot, key); session != nil {
		return &managedMCPClient{session: session, borrowed: true}, nil
	}
	httpClient, err := e.httpClientWithAuthorization(ctx, resolved, authorizationHeader)
	if err != nil {
		return nil, err
	}
	// RPC deadlines come from the caller; bounded dial and response-header timeouts are
	// retained. The SDK owns the notification stream until the session is closed.
	httpClient.Timeout = 0
	session, err := connectMCPClientWithCatalogChanges(ctx, &mcpsdk.StreamableClientTransport{
		Endpoint: strings.TrimSpace(resolved.Server.URL), HTTPClient: httpClient,
	}, func(context.Context, *mcpsdk.ToolListChangedRequest) {
		e.invalidateToolListing(key)
		select {
		case catalog.changes <- struct{}{}:
		default:
		}
	})
	if err != nil {
		return nil, err
	}
	capabilities := session.InitializeResult().Capabilities
	if capabilities == nil || capabilities.Tools == nil || !capabilities.Tools.ListChanged {
		return &managedMCPClient{session: session}, nil
	}
	kept, retired := catalog.store(slot, mcpCatalogHTTPEntry{
		key: key, source: source, session: session, usedAt: time.Now(),
	})
	if err := closeMCPClient(retired); err != nil {
		return nil, fmt.Errorf("mcp: retire HTTP catalog subscription: %w", err)
	}
	return &managedMCPClient{session: kept, borrowed: true}, nil
}

func (c *mcpCatalogHTTP) load(slot, key string) *mcpsdk.ClientSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, found := c.sessions[slot]
	if !found || entry.key != key {
		return nil
	}
	entry.usedAt = time.Now()
	c.sessions[slot] = entry
	return entry.session
}

func (c *mcpCatalogHTTP) store(
	slot string, entry mcpCatalogHTTPEntry,
) (kept *mcpsdk.ClientSession, retired *managedMCPClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return entry.session, &managedMCPClient{session: entry.session}
	}
	if current, found := c.sessions[slot]; found {
		if current.key == entry.key {
			return current.session, &managedMCPClient{session: entry.session}
		}
		retired = &managedMCPClient{session: current.session}
	} else if len(c.sessions) >= defaultToolNormalizationCacheCapacity {
		var oldest string
		usedAt := entry.usedAt
		for candidate, previous := range c.sessions {
			if oldest == "" || previous.usedAt.Before(usedAt) {
				oldest, usedAt = candidate, previous.usedAt
			}
		}
		retired = &managedMCPClient{session: c.sessions[oldest].session}
		delete(c.sessions, oldest)
	}
	c.sessions[slot] = entry
	return entry.session, retired
}

func (c *mcpCatalogHTTP) forget(session *mcpsdk.ClientSession) error {
	return c.retire(func(entry mcpCatalogHTTPEntry) bool { return entry.session == session }, false)
}

func (c *mcpCatalogHTTP) retire(match func(mcpCatalogHTTPEntry) bool, closeAll bool) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	c.closed = c.closed || closeAll
	var retired []*mcpsdk.ClientSession
	for slot, entry := range c.sessions {
		if closeAll || match(entry) {
			retired = append(retired, entry.session)
			delete(c.sessions, slot)
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
	return e.catalogHTTP.retire(nil, true)
}

// CatalogChanges signals that at least one HTTP subscription invalidated its listing.
func (e *CallExecutor) CatalogChanges() <-chan struct{} {
	if e.catalogHTTP == nil {
		return nil
	}
	return e.catalogHTTP.changes
}

// RetainCatalogSubscriptions closes notification streams outside the active consented scopes.
func (e *CallExecutor) RetainCatalogSubscriptions(sources map[toolspkg.SourceRef]struct{}) error {
	active := make(map[toolspkg.SourceRef]bool, len(sources))
	for source := range sources {
		active[toolspkg.MCPDeclarationSource(source)] = true
	}
	return e.catalogHTTP.retire(func(entry mcpCatalogHTTPEntry) bool {
		return !active[toolspkg.MCPDeclarationSource(entry.source)]
	}, false)
}
