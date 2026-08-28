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

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/tools"
)

const (
	hostedMCPKey = "mcp"
)

const (
	// HostedServerName is the single stdio MCP server name injected into ACP sessions.
	HostedServerName = "compozy-hosted-tools"
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
	ErrHostedRunRequired       = errors.New("mcp: hosted MCP active run identity is required")
	ErrHostedPeerInvalid       = errors.New("mcp: hosted MCP peer validation failed")
	ErrHostedBinaryInvalid     = errors.New("mcp: hosted MCP binary validation failed")
	ErrHostedRegistryRequired  = errors.New("mcp: hosted MCP registry is required")
	ErrHostedProjectionChanged = errors.New("mcp: hosted MCP projection changed")
)

// HostedRegistry resolves the daemon-owned tool registry at call time.
type HostedRegistry func() tools.Registry

// HostedProjectionGeneration identifies one complete authoritative projection state.
// Returning known=false keeps projection reads uncached.
type HostedProjectionGeneration func(context.Context, tools.Scope) (generation string, known bool)

// HostedConfig configures hosted MCP launch and bind validation.
type HostedConfig struct {
	Enabled              bool
	BindNonceTTL         time.Duration
	ExpectedBinary       string
	HomePaths            compozyconfig.HomePaths
	Registry             HostedRegistry
	ProjectionGeneration HostedProjectionGeneration
	Logger               *slog.Logger
	Now                  func() time.Time
	NonceReader          func([]byte) error
}

// HostedService owns session-scoped hosted MCP launch and bind lifecycle.
type HostedService struct {
	mu sync.Mutex

	enabled              bool
	bindNonceTTL         time.Duration
	expectedBinary       string
	homePaths            compozyconfig.HomePaths
	registry             HostedRegistry
	projectionGeneration HostedProjectionGeneration
	logger               *slog.Logger
	now                  func() time.Time
	nonceReader          func([]byte) error

	launches        map[string]*hostedLaunchRecord
	binds           map[string]*hostedBindRecord
	projectionCache hostedProjectionCache
}

// HostedLaunchRequest describes one session-bound MCP launch.
type HostedLaunchRequest struct {
	SessionID   string
	ProfileID   string
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
	profileID     string
	workspaceID   string
	agentName     string
	nonceHash     string
	expiresAt     time.Time
	armed         bool
	expectedBin   string
	createdAt     time.Time
	established   bool
	correlationID string
	runID         string
	generation    int64
	bindID        string
}

type hostedBindRecord struct {
	bindID        string
	sessionID     string
	profileID     string
	workspaceID   string
	agentName     string
	expectedBin   string
	peer          PeerInfo
	createdAt     time.Time
	correlationID string
	runID         string
	generation    int64
}

// NewHostedService builds a hosted MCP lifecycle service.
func NewHostedService(cfg *HostedConfig) (*HostedService, error) {
	if cfg == nil {
		return nil, errors.New("mcp: hosted MCP config is required")
	}
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
		ttl = compozyconfig.DefaultHostedMCPBindNonceTTLSeconds * time.Second
	}
	expected, err := normalizeExecutablePath(cfg.ExpectedBinary)
	if err != nil {
		return nil, fmt.Errorf("mcp: normalize hosted MCP binary: %w", err)
	}
	return &HostedService{
		enabled:              cfg.Enabled,
		bindNonceTTL:         ttl,
		expectedBinary:       expected,
		homePaths:            cfg.HomePaths,
		registry:             cfg.Registry,
		projectionGeneration: cfg.ProjectionGeneration,
		logger:               cfg.Logger,
		now:                  now,
		nonceReader:          nonceReader,
		launches:             make(map[string]*hostedLaunchRecord),
		binds:                make(map[string]*hostedBindRecord),
		projectionCache:      newHostedProjectionCache(),
	}, nil
}

// Launch mints a session-bound, single-use hosted MCP launch record.
func (s *HostedService) Launch(ctx context.Context, req HostedLaunchRequest) (compozyconfig.MCPServer, error) {
	if err := ctxErr(ctx); err != nil {
		return compozyconfig.MCPServer{}, err
	}
	if s == nil || !s.enabled {
		return compozyconfig.MCPServer{}, ErrHostedDisabled
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return compozyconfig.MCPServer{}, ErrHostedSessionRequired
	}
	nonce, err := s.randomToken(hostedNonceBytes)
	if err != nil {
		return compozyconfig.MCPServer{}, fmt.Errorf("mcp: mint hosted MCP nonce: %w", err)
	}
	correlation, err := s.randomToken(hostedBindBytes)
	if err != nil {
		return compozyconfig.MCPServer{}, fmt.Errorf("mcp: mint hosted MCP correlation id: %w", err)
	}
	now := s.now().UTC()
	record := &hostedLaunchRecord{
		sessionID:     sessionID,
		profileID:     strings.TrimSpace(req.ProfileID),
		workspaceID:   strings.TrimSpace(req.WorkspaceID),
		agentName:     strings.TrimSpace(req.AgentName),
		nonceHash:     tokenHash(nonce),
		expectedBin:   s.expectedBinary,
		createdAt:     now,
		correlationID: correlation,
	}
	s.mu.Lock()
	releasedBindIDs := make([]string, 0)
	for bindID, bind := range s.binds {
		if bind != nil && bind.sessionID == sessionID {
			delete(s.binds, bindID)
			releasedBindIDs = append(releasedBindIDs, bindID)
		}
	}
	s.launches[sessionID] = record
	s.mu.Unlock()
	for _, bindID := range releasedBindIDs {
		s.projectionCache.remove(bindID)
	}

	env := map[string]string{}
	if home := strings.TrimSpace(s.homePaths.HomeDir); home != "" {
		env["COMPOZY_HOME"] = home
	}
	return compozyconfig.MCPServer{
		Name:      HostedServerName,
		Transport: compozyconfig.MCPServerTransportStdio,
		Command:   s.expectedBinary,
		Args:      []string{"tool", hostedMCPKey, "--session", sessionID, "--bind-nonce", nonce},
		Env:       env,
	}, nil
}

// ArmLaunch starts the first-bind window after the ACP provider has initialized.
func (s *HostedService) ArmLaunch(ctx context.Context, sessionID string) (err error) {
	sessionID = strings.TrimSpace(sessionID)
	target := s.launchRecord(sessionID)
	defer func() {
		if err != nil {
			s.discardUnarmedLaunch(sessionID, target)
		}
	}()

	if err := ctxErr(ctx); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return ErrHostedDisabled
	}
	if sessionID == "" {
		return ErrHostedSessionRequired
	}

	now := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.launches[sessionID]
	if record == nil || record != target {
		return ErrHostedNonceInvalid
	}
	if record.armed {
		return nil
	}
	record.expiresAt = now.Add(s.bindNonceTTL)
	record.armed = true
	return nil
}

func (s *HostedService) launchRecord(sessionID string) *hostedLaunchRecord {
	if s == nil || sessionID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.launches[sessionID]
}

func (s *HostedService) discardUnarmedLaunch(sessionID string, target *hostedLaunchRecord) {
	if s == nil || sessionID == "" || target == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if record := s.launches[sessionID]; record == target && !record.armed {
		delete(s.launches, sessionID)
	}
}

// CancelLaunch removes all hosted state for a failed session start.
func (s *HostedService) CancelLaunch(sessionID string) {
	s.ReleaseSession(sessionID)
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
	releasedBindIDs := make([]string, 0)
	for bindID, record := range s.binds {
		if record.sessionID == sessionID {
			delete(s.binds, bindID)
			releasedBindIDs = append(releasedBindIDs, bindID)
		}
	}
	s.mu.Unlock()
	for _, bindID := range releasedBindIDs {
		s.projectionCache.remove(bindID)
	}
}
