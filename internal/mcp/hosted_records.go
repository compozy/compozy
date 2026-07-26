package mcp

import (
	"context"

	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"fmt"

	"os"

	"slices"
	"strings"

	"github.com/compozy/agh/internal/tools"
)

func (s *HostedService) consumeLaunch(
	sessionID string,
	nonce string,
	bindID string,
	peer PeerInfo,
) (*hostedBindRecord, error) {
	now := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	launch, ok := s.launches[sessionID]
	if !ok || launch == nil || launch.consumed || !constantHashEqual(launch.nonceHash, tokenHash(nonce)) {
		return nil, ErrHostedNonceInvalid
	}
	if !launch.expiresAt.After(now) {
		delete(s.launches, sessionID)
		return nil, ErrHostedNonceExpired
	}
	launch.consumed = true
	delete(s.launches, sessionID)
	record := &hostedBindRecord{
		bindID:        bindID,
		sessionID:     launch.sessionID,
		workspaceID:   launch.workspaceID,
		agentName:     launch.agentName,
		expectedBin:   launch.expectedBin,
		peer:          peer,
		createdAt:     now,
		correlationID: launch.correlationID,
	}
	s.binds[bindID] = record
	return record.clone(), nil
}

func (s *HostedService) recordForBind(
	ctx context.Context,
	bindID string,
	peer PeerInfo,
) (*hostedBindRecord, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	if s == nil || !s.enabled {
		return nil, ErrHostedDisabled
	}
	if strings.TrimSpace(bindID) == "" {
		return nil, ErrHostedBindRequired
	}
	if err := s.validatePeer(peer); err != nil {
		return nil, err
	}
	s.mu.Lock()
	record := s.binds[strings.TrimSpace(bindID)]
	s.mu.Unlock()
	if record == nil {
		return nil, ErrHostedBindNotFound
	}
	if err := record.validatePeer(peer); err != nil {
		return nil, err
	}
	return record.clone(), nil
}

func (s *HostedService) projection(ctx context.Context, record *hostedBindRecord) (HostedProjectionResponse, error) {
	if record == nil {
		return HostedProjectionResponse{}, ErrHostedBindNotFound
	}
	registry := s.currentRegistry()
	if registry == nil {
		return HostedProjectionResponse{}, ErrHostedRegistryRequired
	}
	views, err := registry.List(ctx, record.scope())
	if err != nil {
		return HostedProjectionResponse{}, err
	}
	slices.SortFunc(views, func(left, right tools.ToolView) int {
		return strings.Compare(left.Descriptor.ID.String(), right.Descriptor.ID.String())
	})
	return HostedProjectionResponse{
		Tools:  cloneToolViews(views),
		Digest: hostedProjectionDigest(views),
	}, nil
}

func (s *HostedService) validatePeer(peer PeerInfo) error {
	if !peer.Supported {
		return fmt.Errorf("%w: unsupported peer credential inspection", ErrHostedPeerInvalid)
	}
	if peer.PID <= 0 || peer.UID < 0 {
		return fmt.Errorf("%w: missing peer credentials", ErrHostedPeerInvalid)
	}
	if currentUID := os.Getuid(); currentUID >= 0 && peer.UID != currentUID {
		return fmt.Errorf("%w: uid mismatch", ErrHostedPeerInvalid)
	}
	matches, err := sameExecutablePath(s.expectedBinary, peer.ExecutablePath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHostedBinaryInvalid, err)
	}
	if !matches {
		return fmt.Errorf("%w: peer executable mismatch", ErrHostedBinaryInvalid)
	}
	return nil
}

func (s *HostedService) currentRegistry() tools.Registry {
	if s == nil || s.registry == nil {
		return nil
	}
	return s.registry()
}

func (s *HostedService) randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if err := s.nonceReader(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (r *hostedBindRecord) scope() tools.Scope {
	if r == nil {
		return tools.Scope{}
	}
	return tools.Scope{
		SessionID:   r.sessionID,
		WorkspaceID: r.workspaceID,
		AgentName:   r.agentName,
	}
}

func (r *hostedBindRecord) clone() *hostedBindRecord {
	if r == nil {
		return nil
	}
	cloned := *r
	return &cloned
}

func (r *hostedBindRecord) validatePeer(peer PeerInfo) error {
	if r == nil {
		return ErrHostedBindNotFound
	}
	if peer.PID != r.peer.PID || peer.UID != r.peer.UID {
		return fmt.Errorf("%w: peer credential changed", ErrHostedPeerInvalid)
	}
	matches, err := sameExecutablePath(r.expectedBin, peer.ExecutablePath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHostedBinaryInvalid, err)
	}
	if !matches {
		return fmt.Errorf("%w: peer executable mismatch", ErrHostedBinaryInvalid)
	}
	return nil
}

func hostedProjectionDigest(views []tools.ToolView) string {
	payload, err := json.Marshal(views)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
