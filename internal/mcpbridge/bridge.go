// Package mcpbridge serves a stdio MCP server's tools to an ACP agent over
// loopback streamable HTTP. Some agents (GitHub Copilot CLI) accept only
// remote MCP servers in session/new, so a per-session stdio server such as
// the hosted Compozy tool proxy is relayed through a loopback listener
// guarded by a per-bridge bearer token.
package mcpbridge

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/version"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	loopbackListenAddress = "127.0.0.1:0"
	endpointPath          = "/mcp"
	tokenBytes            = 32
	readHeaderTimeout     = 5 * time.Second
	idleTimeout           = 60 * time.Second
	shutdownTimeout       = 5 * time.Second
	childExitTimeout      = 5 * time.Second
	toolSyncTimeout       = 10 * time.Second
	handshakeTimeout      = 30 * time.Second
	stderrTailBytes       = 2048
)

// Spec describes the stdio MCP server one bridge launches and relays.
type Spec struct {
	Name    string
	Command string
	Args    []string
	// Env is the complete child environment, already filtered by the caller.
	Env []string
	Dir string
}

// Bridge relays one stdio MCP server's tools over a loopback HTTP endpoint.
// Close releases the listener and the child; it is safe to call more than once.
type Bridge struct {
	name   string
	url    string
	token  string
	logger *slog.Logger

	client   *sdkmcp.ClientSession
	child    *stdioChild
	server   *sdkmcp.Server
	http     *http.Server
	serveErr chan error

	syncMu sync.Mutex
	tools  map[string]struct{}

	progress progressRelay

	closeOnce sync.Once
	closeErr  error
}

// Start launches spec as a stdio MCP client, mirrors its tools onto an HTTP
// MCP server, and listens on an ephemeral loopback port. The returned bridge
// owns the child process and listener until Close.
func Start(ctx context.Context, spec Spec, logger *slog.Logger) (_ *Bridge, err error) {
	if ctx == nil {
		return nil, errors.New("mcpbridge: context is required")
	}
	name := strings.TrimSpace(spec.Name)
	command := strings.TrimSpace(spec.Command)
	if name == "" || command == "" {
		return nil, errors.New("mcpbridge: server name and command are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	bridge := &Bridge{
		name:     name,
		token:    token,
		logger:   logger.With("mcp_server", name),
		serveErr: make(chan error, 1),
		tools:    make(map[string]struct{}),
		progress: progressRelay{calls: make(map[string]progressTarget)},
	}
	bridge.progress.logger = bridge.logger
	defer func() {
		if err != nil {
			err = errors.Join(err, bridge.Close())
		}
	}()

	bridge.server = sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: name, Version: version.Current().Version},
		&sdkmcp.ServerOptions{
			Capabilities: &sdkmcp.ServerCapabilities{Tools: &sdkmcp.ToolCapabilities{ListChanged: true}},
		},
	)
	if err := bridge.connectChild(ctx, spec); err != nil {
		return nil, err
	}
	syncCtx, cancelSync := context.WithTimeout(ctx, toolSyncTimeout)
	err = bridge.syncTools(syncCtx, bridge.client)
	cancelSync()
	if err != nil {
		return nil, err
	}
	if err := bridge.listen(ctx); err != nil {
		return nil, err
	}
	return bridge, nil
}

// Name is the MCP server name the agent sees.
func (b *Bridge) Name() string { return b.name }

// URL is the loopback streamable HTTP endpoint.
func (b *Bridge) URL() string { return b.url }

// AuthorizationHeader is the header value the agent must send. It is a
// secret: never log it or persist it outside the session request.
func (b *Bridge) AuthorizationHeader() string { return "Bearer " + b.token }

func (b *Bridge) connectChild(ctx context.Context, spec Spec) error {
	child, err := startStdioChild(ctx, b.name, spec)
	if err != nil {
		return err
	}
	b.child = child
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "compozy-mcp-bridge", Version: version.Current().Version},
		&sdkmcp.ClientOptions{
			Capabilities: &sdkmcp.ClientCapabilities{},
			// The handler can run before Connect returns, so it uses the request's session.
			ToolListChangedHandler: func(ctx context.Context, req *sdkmcp.ToolListChangedRequest) {
				syncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), toolSyncTimeout)
				defer cancel()
				if err := b.syncTools(syncCtx, req.Session); err != nil {
					b.logger.Warn("mcpbridge: resync tools after list change", "error", err)
				}
			},
			ProgressNotificationHandler: func(ctx context.Context, req *sdkmcp.ProgressNotificationClientRequest) {
				b.progress.relay(ctx, req.Params)
			},
		},
	)
	// The deadline bounds only the initialize handshake; the session outlives it.
	connectCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	session, err := client.Connect(connectCtx, child.transport(), nil)
	cancel()
	if err != nil {
		if stderr := child.stderrSummary(); stderr != "" {
			return fmt.Errorf("mcpbridge: connect stdio server %q: %w (stderr: %s)", b.name, err, stderr)
		}
		return fmt.Errorf("mcpbridge: connect stdio server %q: %w", b.name, err)
	}
	b.client = session
	return nil
}

// syncTools mirrors the child's current tool list onto the HTTP server.
func (b *Bridge) syncTools(ctx context.Context, session *sdkmcp.ClientSession) error {
	if session == nil {
		return fmt.Errorf("mcpbridge: list tools from %q: no client session", b.name)
	}
	b.syncMu.Lock()
	defer b.syncMu.Unlock()
	current := make(map[string]struct{})
	// Stage the full listing so a failed page never leaves a mixed catalog behind.
	listed := make([]*sdkmcp.Tool, 0)
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return fmt.Errorf("mcpbridge: list tools from %q: %w", b.name, err)
		}
		if tool == nil || strings.TrimSpace(tool.Name) == "" {
			continue
		}
		if !objectSchema(tool.InputSchema) {
			b.logger.Warn("mcpbridge: skip tool without an object input schema", "tool", tool.Name)
			continue
		}
		current[tool.Name] = struct{}{}
		listed = append(listed, tool)
	}
	for _, tool := range listed {
		b.server.AddTool(tool, b.forward(tool.Name))
	}
	stale := make([]string, 0)
	for name := range b.tools {
		if _, ok := current[name]; !ok {
			stale = append(stale, name)
		}
	}
	if len(stale) > 0 {
		b.server.RemoveTools(stale...)
	}
	b.tools = current
	return nil
}

func (b *Bridge) forward(name string) sdkmcp.ToolHandler {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		params := &sdkmcp.CallToolParams{Name: name}
		if req == nil || req.Params == nil {
			return b.client.CallTool(ctx, params)
		}
		params.Meta = maps.Clone(req.Params.Meta)
		if len(req.Params.Arguments) > 0 {
			params.Arguments = req.Params.Arguments
		}
		if token := req.Params.GetProgressToken(); token != nil && req.Session != nil {
			// Child progress keeps long calls alive on the agent side; route it back
			// to this HTTP request under the token the agent chose.
			relayToken, release := b.progress.register(ctx, req.Session, token)
			defer release()
			params.SetProgressToken(relayToken)
		}
		return b.client.CallTool(ctx, params)
	}
}

func (b *Bridge) listen(ctx context.Context) error {
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", loopbackListenAddress)
	if err != nil {
		return fmt.Errorf("mcpbridge: listen on loopback: %w", err)
	}
	b.url = "http://" + listener.Addr().String() + endpointPath
	handler := sdkmcp.NewStreamableHTTPHandler(
		func(*http.Request) *sdkmcp.Server { return b.server },
		&sdkmcp.StreamableHTTPOptions{
			// SSE responses (not JSON) carry progress notifications for in-flight calls.
			Stateless:                    true,
			Logger:                       b.logger,
			PropagateRequestCancellation: true,
		},
	)
	mux := http.NewServeMux()
	mux.Handle(endpointPath, requireBearer(b.token, handler))
	b.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}
	go func() {
		serveErr := b.http.Serve(listener)
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		b.serveErr <- serveErr
	}()
	return nil
}

// Close stops the listener, then the child. It returns the first teardown
// errors and is idempotent.
func (b *Bridge) Close() error {
	if b == nil {
		return nil
	}
	b.closeOnce.Do(func() {
		var errs []error
		if b.http != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			err := b.http.Shutdown(shutdownCtx)
			cancel()
			if errors.Is(err, context.DeadlineExceeded) {
				// Lingering connections (an agent's open stream, or a dialed but unused
				// keep-alive connection) must not keep the listener alive past teardown.
				err = b.http.Close()
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("mcpbridge: shut down %q listener: %w", b.name, err))
			}
			if err := <-b.serveErr; err != nil {
				errs = append(errs, fmt.Errorf("mcpbridge: serve %q: %w", b.name, err))
			}
		}
		if b.client != nil {
			if err := ignoreClosed(b.client.Close()); err != nil {
				errs = append(errs, fmt.Errorf("mcpbridge: close stdio server %q session: %w", b.name, err))
			}
		}
		// Closing the session ends stdin; the child owner reaps the process group.
		if err := b.child.close(); err != nil {
			errs = append(errs, fmt.Errorf("mcpbridge: stop stdio server %q: %w", b.name, err))
		}
		b.closeErr = errors.Join(errs...)
	})
	return b.closeErr
}

func requireBearer(token string, next http.Handler) http.Handler {
	expected := []byte("Bearer " + token)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		provided := []byte(request.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(provided, expected) != 1 {
			writer.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func newToken() (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("mcpbridge: generate bearer token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func objectSchema(schema any) bool {
	typed, ok := schema.(map[string]any)
	if !ok {
		return false
	}
	kind, ok := typed["type"].(string)
	return ok && kind == "object"
}
