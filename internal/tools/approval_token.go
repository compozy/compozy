package tools

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const approvalTokenBytes = 32

// ApprovalRequest describes one concrete local approval-token issuance request.
type ApprovalRequest struct {
	ToolID      ToolID          `json:"tool_id"`
	SessionID   string          `json:"session_id"`
	WorkspaceID string          `json:"workspace_id,omitempty"`
	AgentName   string          `json:"agent_name,omitempty"`
	Input       json.RawMessage `json:"input,omitempty"`
	InputDigest string          `json:"input_digest,omitempty"`
}

// ApprovalTokenGrant is the raw single-use approval token returned only to authenticated local callers.
type ApprovalTokenGrant struct {
	ApprovalToken string    `json:"approval_token"`
	ExpiresAt     time.Time `json:"expires_at"`
	ToolID        ToolID    `json:"tool_id"`
	InputDigest   string    `json:"input_digest"`
}

// ApprovalTokenIssuer mints local single-use approval references.
type ApprovalTokenIssuer interface {
	CreateToolApproval(ctx context.Context, scope Scope, req ApprovalRequest) (ApprovalTokenGrant, error)
}

// ApprovalTokenConsumer validates and consumes local approval references.
type ApprovalTokenConsumer interface {
	ConsumeToolApproval(ctx context.Context, scope Scope, call CallRequest) error
}

// ApprovalTokenStore keeps local approval references in daemon memory only.
type ApprovalTokenStore struct {
	mu     sync.Mutex
	ttl    time.Duration
	now    func() time.Time
	random io.Reader
	active map[[sha256.Size]byte]approvalTokenRecord
	used   map[[sha256.Size]byte]time.Time
}

type approvalTokenRecord struct {
	toolID      ToolID
	sessionID   string
	workspaceID string
	agentName   string
	inputDigest string
	expiresAt   time.Time
}

// ApprovalTokenStoreOption customizes an in-memory approval token store.
type ApprovalTokenStoreOption func(*ApprovalTokenStore)

// WithApprovalTokenClock overrides the approval-token clock for tests.
func WithApprovalTokenClock(now func() time.Time) ApprovalTokenStoreOption {
	return func(store *ApprovalTokenStore) {
		if now != nil {
			store.now = now
		}
	}
}

// WithApprovalTokenRandom overrides the random source for tests.
func WithApprovalTokenRandom(random io.Reader) ApprovalTokenStoreOption {
	return func(store *ApprovalTokenStore) {
		if random != nil {
			store.random = random
		}
	}
}

// NewApprovalTokenStore builds a daemon-memory approval token store.
func NewApprovalTokenStore(ttl time.Duration, opts ...ApprovalTokenStoreOption) *ApprovalTokenStore {
	if ttl <= 0 {
		ttl = 120 * time.Second
	}
	store := &ApprovalTokenStore{
		ttl: ttl,
		now: func() time.Time {
			return time.Now().UTC()
		},
		random: rand.Reader,
		active: make(map[[sha256.Size]byte]approvalTokenRecord),
		used:   make(map[[sha256.Size]byte]time.Time),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(store)
		}
	}
	return store
}

var _ ApprovalTokenIssuer = (*ApprovalTokenStore)(nil)
var _ ApprovalTokenConsumer = (*ApprovalTokenStore)(nil)

// CreateToolApproval mints a single-use approval token bound to one invocation.
func (s *ApprovalTokenStore) CreateToolApproval(
	ctx context.Context,
	scope Scope,
	req ApprovalRequest,
) (ApprovalTokenGrant, error) {
	if s == nil {
		return ApprovalTokenGrant{}, approvalTokenError(
			req.ToolID,
			"tool approval channel is unavailable",
			ReasonApprovalUnreachable,
		)
	}
	if err := contextErr(ctx, req.ToolID); err != nil {
		return ApprovalTokenGrant{}, err
	}
	if err := req.ToolID.Validate(); err != nil {
		return ApprovalTokenGrant{}, invalidInputError(req.ToolID, "tool id is invalid", err)
	}
	normalizedReq, err := normalizeApprovalRequest(scope, req)
	if err != nil {
		return ApprovalTokenGrant{}, invalidInputError(req.ToolID, "approval scope is invalid", err)
	}
	req = normalizedReq
	if strings.TrimSpace(req.SessionID) == "" {
		return ApprovalTokenGrant{}, invalidInputError(
			req.ToolID,
			"session_id is required for tool approval",
			NewValidationError("session_id", ReasonApprovalRequired, "session_id is required"),
		)
	}
	inputDigest, err := ApprovalInputDigest(req.Input, req.InputDigest)
	if err != nil {
		return ApprovalTokenGrant{}, invalidInputError(req.ToolID, "input digest is invalid", err)
	}
	token, err := randomApprovalToken(s.random)
	if err != nil {
		return ApprovalTokenGrant{}, NewToolError(
			ErrorCodeBackendFailed,
			req.ToolID,
			"tool approval token generation failed",
			fmt.Errorf("%w: %w", ErrToolBackendFailed, err),
			ReasonApprovalUnreachable,
		)
	}
	hash := approvalTokenHash(token)
	now := s.now().UTC()
	record := approvalTokenRecord{
		toolID:      req.ToolID,
		sessionID:   strings.TrimSpace(req.SessionID),
		workspaceID: strings.TrimSpace(req.WorkspaceID),
		agentName:   strings.TrimSpace(req.AgentName),
		inputDigest: inputDigest,
		expiresAt:   now.Add(s.ttl),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(now)
	s.active[hash] = record
	delete(s.used, hash)
	return ApprovalTokenGrant{
		ApprovalToken: token,
		ExpiresAt:     record.expiresAt,
		ToolID:        req.ToolID,
		InputDigest:   inputDigest,
	}, nil
}

// ConsumeToolApproval validates and consumes a single-use approval token.
func (s *ApprovalTokenStore) ConsumeToolApproval(ctx context.Context, scope Scope, call CallRequest) error {
	if err := contextErr(ctx, call.ToolID); err != nil {
		return err
	}
	if s == nil {
		return approvalTokenError(call.ToolID, "tool approval channel is unavailable", ReasonApprovalUnreachable)
	}
	token := strings.TrimSpace(call.ApprovalToken)
	if token == "" {
		return approvalTokenError(call.ToolID, "tool approval token is required", ReasonApprovalTokenMissing)
	}
	call, err := normalizeCallRequest(scope, call)
	if err != nil {
		return err
	}
	inputDigest, err := ApprovalInputDigest(call.Input, "")
	if err != nil {
		return invalidInputError(call.ToolID, "approval input digest is invalid", err)
	}
	hash := approvalTokenHash(token)
	now := s.now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.active[hash]
	if !ok {
		s.pruneExpiredLocked(now)
		if _, replayed := s.used[hash]; replayed {
			return approvalTokenError(call.ToolID, "tool approval token was already used", ReasonApprovalTokenReplayed)
		}
		return approvalTokenError(
			call.ToolID,
			"tool approval token does not match this invocation",
			ReasonApprovalTokenMismatch,
		)
	}
	if !now.Before(record.expiresAt) {
		delete(s.active, hash)
		return approvalTokenError(call.ToolID, "tool approval token expired", ReasonApprovalTokenExpired)
	}
	if !approvalTokenRecordMatches(record, call, inputDigest) {
		return approvalTokenError(
			call.ToolID,
			"tool approval token does not match this invocation",
			ReasonApprovalTokenMismatch,
		)
	}
	delete(s.active, hash)
	s.used[hash] = record.expiresAt
	return nil
}

// ApprovalInputDigest returns the stable digest binding used for local approvals.
func ApprovalInputDigest(input json.RawMessage, suppliedDigest string) (string, error) {
	digest := strings.TrimSpace(suppliedDigest)
	if len(input) == 0 {
		if digest != "" {
			return digest, nil
		}
		input = json.RawMessage(`{}`)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, input); err != nil {
		return "", NewValidationError("input", ReasonSchemaInvalid, "input must be valid JSON")
	}
	sum := sha256.Sum256(compact.Bytes())
	computed := "sha256:" + hex.EncodeToString(sum[:])
	if digest != "" && subtle.ConstantTimeCompare([]byte(digest), []byte(computed)) != 1 {
		return "", NewValidationError("input_digest", ReasonApprovalTokenMismatch, "input digest does not match input")
	}
	return computed, nil
}

func normalizeApprovalRequest(scope Scope, req ApprovalRequest) (ApprovalRequest, error) {
	sessionID, err := approvalScopeValue("session_id", scope.SessionID, req.SessionID)
	if err != nil {
		return ApprovalRequest{}, err
	}
	workspaceID, err := approvalScopeValue("workspace_id", scope.WorkspaceID, req.WorkspaceID)
	if err != nil {
		return ApprovalRequest{}, err
	}
	agentName, err := approvalScopeValue("agent_name", scope.AgentName, req.AgentName)
	if err != nil {
		return ApprovalRequest{}, err
	}
	req.SessionID = sessionID
	req.WorkspaceID = workspaceID
	req.AgentName = agentName
	return req, nil
}

func approvalScopeValue(field string, scoped string, requested string) (string, error) {
	scoped = strings.TrimSpace(scoped)
	requested = strings.TrimSpace(requested)
	if scoped != "" && requested != "" && requested != scoped {
		return "", NewValidationError(field, ReasonApprovalTokenMismatch, field+" does not match approval scope")
	}
	if requested != "" {
		return requested, nil
	}
	return scoped, nil
}

func randomApprovalToken(random io.Reader) (string, error) {
	if random == nil {
		return "", errors.New("random source is required")
	}
	buf := make([]byte, approvalTokenBytes)
	if _, err := io.ReadFull(random, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func approvalTokenHash(token string) [sha256.Size]byte {
	return sha256.Sum256([]byte(token))
}

func approvalTokenRecordMatches(record approvalTokenRecord, call CallRequest, inputDigest string) bool {
	if record.toolID != call.ToolID {
		return false
	}
	if strings.TrimSpace(record.sessionID) != strings.TrimSpace(call.SessionID) {
		return false
	}
	if strings.TrimSpace(record.workspaceID) != strings.TrimSpace(call.WorkspaceID) {
		return false
	}
	if strings.TrimSpace(record.agentName) != strings.TrimSpace(call.AgentName) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(record.inputDigest), []byte(inputDigest)) == 1
}

func (s *ApprovalTokenStore) pruneExpiredLocked(now time.Time) {
	for hash, record := range s.active {
		if !now.Before(record.expiresAt) {
			delete(s.active, hash)
		}
	}
	for hash, expiresAt := range s.used {
		if !now.Before(expiresAt) {
			delete(s.used, hash)
		}
	}
}

func approvalTokenError(id ToolID, message string, reason ReasonCode) *ToolError {
	return NewToolError(
		ErrorCodeApprovalRequired,
		id,
		fmt.Sprintf("%s for %q", message, id),
		ErrToolApprovalRequired,
		reason,
		ReasonApprovalRequired,
	)
}
