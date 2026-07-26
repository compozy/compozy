package mcp

import (
	"context"
	"crypto/rand"

	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"strings"
	"sync"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/tools"
)

const (
	hostedMCPKey = "mcp"
)

const (
	// HostedServerName is the single stdio MCP server name injected into ACP sessions.
	HostedServerName = "agh-hosted-tools"
	hostedNonceBytes = 32
	hostedBindBytes  = 24
)

var (
	ErrHostedDisabled          = errors.New("mcp: hosted MCP is disabled")
	ErrHostedSessionRequired   = errors.New("mcp: hosted MCP session id is required")
	ErrHostedNonceRequired     = errors.New("mcp: hosted MCP bind nonce is required")
	ErrHostedNonceInvalid      = errors.New("mcp: hosted MCP bind nonce is invalid")
	ErrHostedNonceExpired      = errors.New("mcp: hosted MCP bind nonce expired")
	ErrHostedBindRequired      = errors.New("mcp: hosted MCP bind id is required")
	ErrHostedBindNotFound      = errors.New("mcp: hosted MCP bind not found")
	ErrHostedPeerInvalid       = errors.New("mcp: hosted MCP peer validation failed")
	ErrHostedBinaryInvalid     = errors.New("mcp: hosted MCP binary validation failed")
	ErrHostedRegistryRequired  = errors.New("mcp: hosted MCP registry is required")
	ErrHostedProjectionChanged = errors.New("mcp: hosted MCP projection changed")
)

// HostedRegistry resolves the daemon-owned tool registry at call time.
type HostedRegistry func() tools.Registry

// HostedConfig configures hosted MCP launch and bind validation.
type HostedConfig struct {
	Enabled        bool
	BindNonceTTL   time.Duration
	ExpectedBinary string
	HomePaths      aghconfig.HomePaths
	Registry       HostedRegistry
	Logger         *slog.Logger
	Now            func() time.Time
	NonceReader    func([]byte) error
}

// HostedService owns session-scoped hosted MCP launch and bind lifecycle.
type HostedService struct {
	mu sync.Mutex

	enabled        bool
	bindNonceTTL   time.Duration
	expectedBinary string
	homePaths      aghconfig.HomePaths
	registry       HostedRegistry
	logger         *slog.Logger
	now            func() time.Time
	nonceReader    func([]byte) error

	launches map[string]*hostedLaunchRecord
	binds    map[string]*hostedBindRecord
}

// HostedLaunchRequest describes one session-bound MCP launch.
type HostedLaunchRequest struct {
	SessionID   string
	WorkspaceID string
	AgentName   string
}

// HostedBindRequest is sent by the stdio proxy immediately after ACP starts it.
type HostedBindRequest struct {
	SessionID string `json:"session_id"`
	Nonce     string `json:"bind_nonce"`
}

// HostedBindResponse proves a hosted proxy has bound to one launch record.
type HostedBindResponse struct {
	BindID string           `json:"bind_id"`
	Scope  tools.Scope      `json:"scope"`
	Tools  []tools.ToolView `json:"tools"`
	Digest string           `json:"digest"`
}

// HostedProjectionResponse is a full session-callable projection snapshot.
type HostedProjectionResponse struct {
	Tools  []tools.ToolView `json:"tools"`
	Digest string           `json:"digest"`
}

// HostedCallRequest invokes one hosted MCP tool through the registry.
type HostedCallRequest struct {
	BindID        string          `json:"bind_id"`
	ToolName      string          `json:"tool_name"`
	ToolCallID    string          `json:"tool_call_id,omitempty"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	Input         json.RawMessage `json:"input,omitempty"`
}

// HostedCallResponse carries the canonical registry result envelope.
type HostedCallResponse struct {
	Result tools.ToolResult `json:"result"`
}

// HostedReleaseRequest releases one active bind record.
type HostedReleaseRequest struct {
	BindID string `json:"bind_id"`
}

type hostedLaunchRecord struct {
	sessionID     string
	workspaceID   string
	agentName     string
	nonceHash     string
	expiresAt     time.Time
	expectedBin   string
	createdAt     time.Time
	consumed      bool
	correlationID string
}

type hostedBindRecord struct {
	bindID        string
	sessionID     string
	workspaceID   string
	agentName     string
	expectedBin   string
	peer          PeerInfo
	createdAt     time.Time
	correlationID string
}

// NewHostedService builds a hosted MCP lifecycle service.
func NewHostedService(cfg HostedConfig) (*HostedService, error) {
	now := cfg.Now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	nonceReader := cfg.NonceReader
	if nonceReader == nil {
		nonceReader = func(dst []byte) error {
			_, err := rand.Read(dst)
			return err
		}
	}
	ttl := cfg.BindNonceTTL
	if ttl <= 0 {
		ttl = aghconfig.DefaultHostedMCPBindNonceTTLSeconds * time.Second
	}
	expected, err := normalizeExecutablePath(cfg.ExpectedBinary)
	if err != nil {
		return nil, fmt.Errorf("mcp: normalize hosted MCP binary: %w", err)
	}
	return &HostedService{
		enabled:        cfg.Enabled,
		bindNonceTTL:   ttl,
		expectedBinary: expected,
		homePaths:      cfg.HomePaths,
		registry:       cfg.Registry,
		logger:         cfg.Logger,
		now:            now,
		nonceReader:    nonceReader,
		launches:       make(map[string]*hostedLaunchRecord),
		binds:          make(map[string]*hostedBindRecord),
	}, nil
}

// Launch mints a session-bound, single-use hosted MCP launch record.
func (s *HostedService) Launch(ctx context.Context, req HostedLaunchRequest) (aghconfig.MCPServer, error) {
	if err := ctxErr(ctx); err != nil {
		return aghconfig.MCPServer{}, err
	}
	if s == nil || !s.enabled {
		return aghconfig.MCPServer{}, ErrHostedDisabled
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return aghconfig.MCPServer{}, ErrHostedSessionRequired
	}
	nonce, err := s.randomToken(hostedNonceBytes)
	if err != nil {
		return aghconfig.MCPServer{}, fmt.Errorf("mcp: mint hosted MCP nonce: %w", err)
	}
	correlation, err := s.randomToken(hostedBindBytes)
	if err != nil {
		return aghconfig.MCPServer{}, fmt.Errorf("mcp: mint hosted MCP correlation id: %w", err)
	}
	now := s.now().UTC()
	record := &hostedLaunchRecord{
		sessionID:     sessionID,
		workspaceID:   strings.TrimSpace(req.WorkspaceID),
		agentName:     strings.TrimSpace(req.AgentName),
		nonceHash:     tokenHash(nonce),
		expiresAt:     now.Add(s.bindNonceTTL),
		expectedBin:   s.expectedBinary,
		createdAt:     now,
		correlationID: correlation,
	}
	s.mu.Lock()
	s.launches[sessionID] = record
	s.mu.Unlock()

	env := map[string]string{}
	if home := strings.TrimSpace(s.homePaths.HomeDir); home != "" {
		env["AGH_HOME"] = home
	}
	return aghconfig.MCPServer{
		Name:      HostedServerName,
		Transport: aghconfig.MCPServerTransportStdio,
		Command:   s.expectedBinary,
		Args:      []string{"tool", hostedMCPKey, "--session", sessionID, "--bind-nonce", nonce},
		Env:       env,
	}, nil
}

// CancelLaunch removes an unbound launch record for a failed session start.
func (s *HostedService) CancelLaunch(sessionID string) {
	if s == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	if record, ok := s.launches[sessionID]; ok && !record.consumed {
		delete(s.launches, sessionID)
	}
	s.mu.Unlock()
}

// ReleaseSession removes all hosted state for a stopped session.
func (s *HostedService) ReleaseSession(sessionID string) {
	if s == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	delete(s.launches, sessionID)
	for bindID, record := range s.binds {
		if record.sessionID == sessionID {
			delete(s.binds, bindID)
		}
	}
	s.mu.Unlock()
}
