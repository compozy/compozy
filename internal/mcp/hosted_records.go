package mcp

import (
	"context"

	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"fmt"

	"os"
	"reflect"

	"slices"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/tools"
)

// bindLaunch consumes one armed launch nonce and gives the resulting binding its own digest memo.
func (s *HostedService) bindLaunch(
	sessionID string,
	nonce string,
	bindID string,
	peer PeerInfo,
) (*hostedBindRecord, error) {
	now := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	launch, ok := s.launches[sessionID]
	if !ok || launch == nil || !constantHashEqual(launch.nonceHash, tokenHash(nonce)) {
		return nil, ErrHostedNonceInvalid
	}
	if !launch.armed {
		return nil, ErrHostedNonceInvalid
	}
	if launch.established {
		return nil, ErrHostedNonceInvalid
	}
	if !launch.expiresAt.After(now) {
		delete(s.launches, sessionID)
		return nil, ErrHostedNonceExpired
	}
	launch.established = true
	launch.bindID = bindID
	record := &hostedBindRecord{
		digestMemo:    new(hostedProjectionDigestMemo),
		bindID:        bindID,
		sessionID:     launch.sessionID,
		profileID:     launch.profileID,
		workspaceID:   launch.workspaceID,
		agentName:     launch.agentName,
		expectedBin:   launch.expectedBin,
		peer:          peer,
		createdAt:     now,
		correlationID: launch.correlationID,
		runID:         launch.runID,
		generation:    launch.generation,
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
	record := s.binds[strings.TrimSpace(bindID)].clone()
	s.mu.Unlock()
	if record == nil {
		return nil, ErrHostedBindNotFound
	}
	if err := record.validatePeer(peer); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *HostedService) projection(ctx context.Context, record *hostedBindRecord) (HostedProjectionResponse, error) {
	if record == nil {
		return HostedProjectionResponse{}, ErrHostedBindNotFound
	}
	registry := s.currentRegistry()
	if registry == nil {
		return HostedProjectionResponse{}, ErrHostedRegistryRequired
	}
	return s.projectionForGeneration(ctx, record, registry)
}

// hostedProjectionResponse returns isolated, ID-sorted views and a digest over their canonical empty form.
func hostedProjectionResponse(views []tools.ToolView, memo *hostedProjectionDigestMemo) HostedProjectionResponse {
	sorted := cloneToolViews(views)
	if len(sorted) == 0 {
		sorted = nil
	}
	slices.SortFunc(sorted, func(left, right tools.ToolView) int {
		return strings.Compare(left.Descriptor.ID.String(), right.Descriptor.ID.String())
	})
	return HostedProjectionResponse{
		Tools:  sorted,
		Digest: memo.digestFor(sorted),
	}
}

type hostedProjectionDigestMemo struct {
	mu     sync.Mutex
	views  []tools.ToolView
	digest string
}

// digestFor synchronizes exact-view digest reuse and never caches a failed encoding.
func (m *hostedProjectionDigestMemo) digestFor(views []tools.ToolView) string {
	if m == nil {
		return hostedProjectionDigest(views)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// Complete equality avoids repeated JSON schema encoding without omitting present or future view fields.
	if m.digest != "" && reflect.DeepEqual(m.views, views) {
		return m.digest
	}
	digest := hostedProjectionDigest(views)
	if digest != "" {
		m.views = cloneToolViews(views)
		m.digest = digest
	}
	return digest
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
		ProfileID:   r.profileID,
		SessionID:   r.sessionID,
		WorkspaceID: r.workspaceID,
		AgentName:   r.agentName,
		RunID:       r.runID,
		Generation:  r.generation,
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
