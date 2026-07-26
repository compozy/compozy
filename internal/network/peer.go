package network

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	sessionpkg "github.com/compozy/agh/internal/session"
)

// LocalPeer is one daemon-local peer joined to one runtime channel.
type LocalPeer struct {
	SessionID         string
	PeerID            string
	WorkspaceID       string
	Channel           string
	PeerCard          PeerCard
	CapabilityCatalog []sessionpkg.NetworkPeerCapability
	JoinedAt          time.Time
}

// PeerInfo is the API-facing snapshot for one daemon-local peer.
type PeerInfo struct {
	SessionID              *string
	PeerID                 string
	WorkspaceID            string
	Channel                string
	Local                  bool
	PeerCard               PeerCard
	CapabilityCatalog      []sessionpkg.NetworkPeerCapability
	CapabilityCatalogKnown bool
	JoinedAt               *time.Time
	PresenceState          PresenceState
}

// ChannelInfo summarizes one active runtime channel.
type ChannelInfo struct {
	WorkspaceID string
	Channel     string
	PeerCount   int
}

// PeerRegistry tracks daemon-local session membership only.
type PeerRegistry struct {
	mu              sync.RWMutex
	now             func() time.Time
	localsByID      map[string]LocalPeer
	localsByChannel map[string]map[string]string
}

// PeerRegistryOption customizes the registry runtime.
type PeerRegistryOption func(*PeerRegistry)

// WithPeerRegistryClock overrides the time source used by the registry.
func WithPeerRegistryClock(now func() time.Time) PeerRegistryOption {
	return func(registry *PeerRegistry) {
		registry.now = now
	}
}

// NewPeerRegistry constructs the in-process membership registry.
func NewPeerRegistry(opts ...PeerRegistryOption) (*PeerRegistry, error) {
	registry := &PeerRegistry{
		now:             func() time.Time { return time.Now().UTC() },
		localsByID:      make(map[string]LocalPeer),
		localsByChannel: make(map[string]map[string]string),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(registry)
		}
	}
	if registry.now == nil {
		return nil, fmt.Errorf("%w: peer registry clock is required", ErrInvalidField)
	}
	return registry, nil
}

// DefaultPeerCard returns the minimal protocol peer card for one peer identifier.
func DefaultPeerCard(peerID string) (PeerCard, error) {
	card := PeerCard{
		PeerID:              strings.TrimSpace(peerID),
		ProfilesSupported:   []string{ProtocolV0},
		Capabilities:        []string{},
		ArtifactsSupported:  []string{},
		TrustModesSupported: []string{},
	}
	normalized, err := normalizePeerCard(card)
	if err != nil {
		return PeerCard{}, err
	}
	return normalized, nil
}

// RegisterLocal upserts one local peer membership keyed by session ID.
func (r *PeerRegistry) RegisterLocal(
	sessionID string,
	workspaceID string,
	channel string,
	card PeerCard,
	joinedAt time.Time,
) (LocalPeer, error) {
	return r.RegisterLocalWithCapabilityCatalog(sessionID, workspaceID, channel, card, nil, joinedAt)
}

// RegisterLocalWithCapabilityCatalog upserts one local membership and its runtime-owned catalog.
func (r *PeerRegistry) RegisterLocalWithCapabilityCatalog(
	sessionID string,
	workspaceID string,
	channel string,
	card PeerCard,
	capabilityCatalog []sessionpkg.NetworkPeerCapability,
	joinedAt time.Time,
) (LocalPeer, error) {
	if r == nil {
		return LocalPeer{}, fmt.Errorf("%w: peer registry is required", ErrInvalidField)
	}
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		return LocalPeer{}, fmt.Errorf("%w: session id is required", ErrMissingField)
	}
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	if err := ValidateWorkspaceID(trimmedWorkspaceID); err != nil {
		return LocalPeer{}, err
	}
	trimmedChannel := strings.TrimSpace(channel)
	if err := ValidateChannel(trimmedChannel); err != nil {
		return LocalPeer{}, err
	}
	normalizedCard, err := normalizePeerCard(card)
	if err != nil {
		return LocalPeer{}, err
	}
	if joinedAt.IsZero() {
		joinedAt = r.now()
	}
	local := LocalPeer{
		SessionID: trimmedSessionID, PeerID: normalizedCard.PeerID,
		WorkspaceID: trimmedWorkspaceID, Channel: trimmedChannel,
		PeerCard: normalizedCard, CapabilityCatalog: cloneNetworkPeerCapabilityCatalog(capabilityCatalog),
		JoinedAt: joinedAt.UTC(),
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	key := peerChannelKey(trimmedWorkspaceID, trimmedChannel)
	if _, ok := r.localsByChannel[key]; !ok {
		r.localsByChannel[key] = make(map[string]string)
	}
	if owner, ok := r.localsByChannel[key][local.PeerID]; ok && owner != trimmedSessionID {
		return LocalPeer{}, fmt.Errorf(
			"%w: local peer_id already registered in channel: %s",
			ErrInvalidField,
			local.PeerID,
		)
	}
	if current, ok := r.localsByID[trimmedSessionID]; ok {
		r.removeLocalIndexesLocked(current)
	}
	r.localsByID[trimmedSessionID] = local
	r.localsByChannel[key][local.PeerID] = trimmedSessionID
	return cloneLocalPeer(local), nil
}

// LeaveLocal removes one local session peer from the registry.
func (r *PeerRegistry) LeaveLocal(sessionID string) (LocalPeer, bool) {
	if r == nil {
		return LocalPeer{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	local, ok := r.localsByID[strings.TrimSpace(sessionID)]
	if !ok {
		return LocalPeer{}, false
	}
	delete(r.localsByID, local.SessionID)
	r.removeLocalIndexesLocked(local)
	return cloneLocalPeer(local), true
}

// LocalBySession resolves one local peer by session ID.
func (r *PeerRegistry) LocalBySession(sessionID string) (LocalPeer, bool) {
	if r == nil {
		return LocalPeer{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	local, ok := r.localsByID[strings.TrimSpace(sessionID)]
	if !ok {
		return LocalPeer{}, false
	}
	return cloneLocalPeer(local), true
}

// LocalByPeer resolves one local peer by workspace, channel, and peer ID.
func (r *PeerRegistry) LocalByPeer(workspaceID string, channel string, peerID string) (LocalPeer, bool) {
	if r == nil {
		return LocalPeer{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	local, ok := r.lookupLocalLocked(
		strings.TrimSpace(workspaceID),
		strings.TrimSpace(channel),
		strings.TrimSpace(peerID),
	)
	if !ok {
		return LocalPeer{}, false
	}
	return cloneLocalPeer(local), true
}

// LocalPeers returns local peers currently joined to one workspace channel.
func (r *PeerRegistry) LocalPeers(workspaceID string, channel string) []LocalPeer {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	sessionIDs := r.localsByChannel[peerChannelKey(workspaceID, channel)]
	peers := make([]LocalPeer, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		peers = append(peers, cloneLocalPeer(r.localsByID[sessionID]))
	}
	sort.Slice(peers, func(i int, j int) bool { return peers[i].SessionID < peers[j].SessionID })
	return peers
}

// ListPeers returns daemon-local peers, optionally filtered by workspace and channel.
func (r *PeerRegistry) ListPeers(workspaceID string, channel string) []PeerInfo {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	trimmedChannel := strings.TrimSpace(channel)
	peers := make([]PeerInfo, 0, len(r.localsByID))
	for _, local := range r.localsByID {
		if trimmedWorkspaceID != "" && local.WorkspaceID != trimmedWorkspaceID {
			continue
		}
		if trimmedChannel != "" && local.Channel != trimmedChannel {
			continue
		}
		peers = append(peers, peerInfoFromLocal(local))
	}
	sortPeerInfos(peers)
	return peers
}

// ListChannels returns active in-process channels plus local member counts.
func (r *PeerRegistry) ListChannels(workspaceID string) []ChannelInfo {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	counts := make(map[string]int)
	for _, local := range r.localsByID {
		if trimmedWorkspaceID != "" && local.WorkspaceID != trimmedWorkspaceID {
			continue
		}
		counts[peerChannelKey(local.WorkspaceID, local.Channel)]++
	}
	channels := make([]ChannelInfo, 0, len(counts))
	for key, count := range counts {
		workspace, channel := splitPeerChannelKey(key)
		channels = append(channels, ChannelInfo{WorkspaceID: workspace, Channel: channel, PeerCount: count})
	}
	sort.Slice(channels, func(i int, j int) bool {
		if channels[i].WorkspaceID != channels[j].WorkspaceID {
			return channels[i].WorkspaceID < channels[j].WorkspaceID
		}
		return channels[i].Channel < channels[j].Channel
	})
	return channels
}

func (r *PeerRegistry) lookupLocalLocked(workspaceID string, channel string, peerID string) (LocalPeer, bool) {
	sessionID, ok := r.localsByChannel[peerChannelKey(workspaceID, channel)][peerID]
	if !ok {
		return LocalPeer{}, false
	}
	local, ok := r.localsByID[sessionID]
	return local, ok
}

func (r *PeerRegistry) removeLocalIndexesLocked(local LocalPeer) {
	key := peerChannelKey(local.WorkspaceID, local.Channel)
	channelEntries := r.localsByChannel[key]
	delete(channelEntries, local.PeerID)
	if len(channelEntries) == 0 {
		delete(r.localsByChannel, key)
	}
}

func normalizePeerCard(card PeerCard) (PeerCard, error) {
	normalized := clonePeerCard(card)
	if err := normalizeAndValidatePeerCard(&normalized); err != nil {
		return PeerCard{}, err
	}
	return normalized, nil
}

func clonePeerCard(card PeerCard) PeerCard {
	cloned := PeerCard{
		PeerID: strings.TrimSpace(card.PeerID), ProfilesSupported: cloneStringList(card.ProfilesSupported),
		Capabilities: cloneStringList(card.Capabilities), ArtifactsSupported: cloneStringList(card.ArtifactsSupported),
		TrustModesSupported: cloneStringList(card.TrustModesSupported), Ext: cloneExtensionMap(card.Ext),
	}
	if card.DisplayName != nil {
		displayName := strings.TrimSpace(*card.DisplayName)
		cloned.DisplayName = &displayName
	}
	return cloned
}

func cloneStringList(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneLocalPeer(local LocalPeer) LocalPeer {
	return LocalPeer{
		SessionID: strings.TrimSpace(local.SessionID), PeerID: strings.TrimSpace(local.PeerID),
		WorkspaceID: strings.TrimSpace(local.WorkspaceID), Channel: strings.TrimSpace(local.Channel),
		PeerCard:          clonePeerCard(local.PeerCard),
		CapabilityCatalog: cloneNetworkPeerCapabilityCatalog(local.CapabilityCatalog),
		JoinedAt:          local.JoinedAt.UTC(),
	}
}

func peerInfoFromLocal(local LocalPeer) PeerInfo {
	sessionID := strings.TrimSpace(local.SessionID)
	joinedAt := local.JoinedAt.UTC()
	return PeerInfo{
		SessionID: &sessionID, PeerID: local.PeerID, WorkspaceID: local.WorkspaceID,
		Channel: local.Channel, Local: true, PeerCard: clonePeerCard(local.PeerCard),
		CapabilityCatalog:      cloneNetworkPeerCapabilityCatalog(local.CapabilityCatalog),
		CapabilityCatalogKnown: true, JoinedAt: &joinedAt, PresenceState: PresenceStateLocal,
	}
}

func sortPeerInfos(peers []PeerInfo) {
	sort.Slice(peers, func(i int, j int) bool {
		if peers[i].WorkspaceID != peers[j].WorkspaceID {
			return peers[i].WorkspaceID < peers[j].WorkspaceID
		}
		if peers[i].Channel != peers[j].Channel {
			return peers[i].Channel < peers[j].Channel
		}
		return peers[i].PeerID < peers[j].PeerID
	})
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func peerChannelKey(workspaceID string, channel string) string {
	return strings.TrimSpace(workspaceID) + "\x00" + strings.TrimSpace(channel)
}

func splitPeerChannelKey(key string) (string, string) {
	workspaceID, channel, ok := strings.Cut(key, "\x00")
	if !ok {
		return "", strings.TrimSpace(key)
	}
	return strings.TrimSpace(workspaceID), strings.TrimSpace(channel)
}
