package core

import (
	"context"

	"errors"
	"fmt"
	"net/http"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (h *BaseHandlers) resolveCreateNetworkChannelRequest(
	ctx context.Context,
	req contract.CreateNetworkChannelRequest,
	resolved *workspacepkg.ResolvedWorkspace,
) (string, string, []string, error) {
	_ = ctx
	if resolved == nil {
		return "", "", nil, NewNetworkValidationError(
			errors.New("workspace is required"),
		)
	}
	channel, err := normalizeNetworkChannel(req.Channel)
	if err != nil {
		return "", "", nil, err
	}
	purpose, err := normalizeNetworkChannelPurpose(req.Purpose)
	if err != nil {
		return "", "", nil, err
	}

	workspaceID := strings.TrimSpace(resolved.ID)
	if workspaceID == "" {
		return "", "", nil, NewNetworkValidationError(
			errors.New("workspace_id is required"),
		)
	}

	agentNames, err := normalizeNetworkAgentNames(req.AgentNames)
	if err != nil {
		return "", "", nil, err
	}
	available := make(map[string]struct{}, len(resolved.Agents))
	for _, agent := range resolved.Agents {
		available[strings.TrimSpace(agent.Name)] = struct{}{}
	}
	for _, agentName := range agentNames {
		if _, ok := available[agentName]; ok {
			continue
		}
		return "", "", nil, fmt.Errorf(
			"%w: %s",
			workspacepkg.ErrAgentNotAvailable,
			agentName,
		)
	}

	return channel, purpose, agentNames, nil
}

func normalizeNetworkChannel(channel string) (string, error) {
	trimmed := strings.TrimSpace(channel)
	if trimmed == "" {
		return "", NewNetworkValidationError(errors.New("channel is required"))
	}
	if err := network.ValidateChannel(trimmed); err != nil {
		return "", err
	}
	return trimmed, nil
}

func normalizeNetworkAgentNames(agentNames []string) ([]string, error) {
	if len(agentNames) == 0 {
		return nil, NewNetworkValidationError(errors.New("agent_names is required"))
	}

	normalized := make([]string, 0, len(agentNames))
	seen := make(map[string]struct{}, len(agentNames))
	for _, raw := range agentNames {
		name := strings.TrimSpace(raw)
		if name == "" {
			return nil, NewNetworkValidationError(errors.New("agent_names entries are required"))
		}
		if _, ok := seen[name]; ok {
			return nil, NewNetworkValidationError(fmt.Errorf("agent_names contains duplicate entry %q", name))
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	return normalized, nil
}

func normalizeNetworkChannelPurpose(purpose string) (string, error) {
	trimmed := strings.TrimSpace(purpose)
	if trimmed == "" {
		return "", NewNetworkValidationError(errors.New("purpose is required"))
	}
	return trimmed, nil
}

func (h *BaseHandlers) networkChannelPayloads(
	ctx context.Context,
	service NetworkService,
	workspaceID string,
) ([]contract.NetworkChannelPayload, error) {
	if h == nil {
		return nil, errors.New("api: handlers are required")
	}
	return NetworkChannelPayloads(ctx, service, h.Sessions, h.NetworkStore, workspaceID)
}

// NetworkChannelPayloads builds the shared runtime channel projection used by transports and tools.
func NetworkChannelPayloads(
	ctx context.Context,
	service NetworkService,
	sessionsManager SessionManager,
	networkStore NetworkStore,
	workspaceID string,
) ([]contract.NetworkChannelPayload, error) {
	aggregates, err := networkChannelAggregates(ctx, service, sessionsManager, networkStore, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("api: build network channel aggregates: %w", err)
	}
	return sortedNetworkChannelPayloads(aggregates), nil
}

// networkChannelAggregates merges live peers with persisted channel/session/message state for one projection pass.
func networkChannelAggregates(
	ctx context.Context,
	service NetworkService,
	sessionsManager SessionManager,
	networkStore NetworkStore,
	workspaceID string,
) (map[string]*networkChannelAggregate, error) {
	if service == nil {
		return nil, errors.New("api: network service is required")
	}
	if networkStore == nil {
		return nil, errors.New("api: network store is required")
	}
	if sessionsManager == nil {
		return nil, errors.New("api: sessions are required")
	}
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	if trimmedWorkspaceID == "" {
		return nil, errors.New("api: network workspace_id is required")
	}
	runtimePeers, err := service.ListPeers(ctx, trimmedWorkspaceID, "")
	if err != nil {
		return nil, fmt.Errorf("api: list network peers: %w", err)
	}
	sessions, err := sessionsManager.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("api: list sessions: %w", err)
	}
	channelMetadata, err := networkStore.ListNetworkChannels(
		ctx,
		store.NetworkChannelQuery{WorkspaceID: trimmedWorkspaceID},
	)
	if err != nil {
		return nil, fmt.Errorf("api: list network channels: %w", err)
	}
	projections, err := listNetworkChannelProjections(ctx, networkStore, trimmedWorkspaceID, "")
	if err != nil {
		return nil, fmt.Errorf("api: list network channel projections: %w", err)
	}

	aggregates := make(map[string]*networkChannelAggregate)
	applyNetworkChannelMetadata(aggregates, trimmedWorkspaceID, channelMetadata)
	applyNetworkChannelSessions(aggregates, trimmedWorkspaceID, sessions)
	applyNetworkChannelPeers(aggregates, runtimePeers)
	applyNetworkChannelProjections(aggregates, projections)
	return aggregates, nil
}

func applyNetworkChannelMetadata(
	aggregates map[string]*networkChannelAggregate,
	workspaceID string,
	metadataEntries []store.NetworkChannelEntry,
) {
	for _, metadata := range metadataEntries {
		metadataCopy := metadata
		aggregate := ensureNetworkChannelAggregate(aggregates, workspaceID, metadata.Channel)
		aggregate.metadata = &metadataCopy
	}
}

func applyNetworkChannelSessions(
	aggregates map[string]*networkChannelAggregate,
	workspaceID string,
	sessions []*session.Info,
) {
	for _, info := range sessions {
		if !networkChannelSessionVisible(info) || strings.TrimSpace(info.WorkspaceID) != workspaceID {
			continue
		}
		aggregate := ensureNetworkChannelAggregate(
			aggregates,
			workspaceID,
			info.NetworkParticipation.ChannelID,
		)
		aggregate.sessionCount++
	}
}

func applyNetworkChannelPeers(
	aggregates map[string]*networkChannelAggregate,
	peers []network.PeerInfo,
) {
	for _, peer := range peers {
		aggregate := ensureNetworkChannelAggregate(aggregates, peer.WorkspaceID, peer.Channel)
		aggregate.peerCount++
		if peer.Local {
			aggregate.localPeerCount++
		}
	}
}

func applyNetworkChannelMessages(
	aggregates map[string]*networkChannelAggregate,
	messages []store.NetworkMessageEntry,
) {
	for _, message := range messages {
		aggregate := ensureNetworkChannelAggregate(aggregates, message.WorkspaceID, message.Channel)
		recordNetworkMessageParticipants(aggregate.recordHistoricalParticipant, message)
		if !isPublicChannelTimelineMessage(message) {
			continue
		}
		if isPresenceMessage(message) {
			aggregate.presenceCount++
			aggregate.lastPresenceAt = laterTimePtr(aggregate.lastPresenceAt, message.Timestamp)
			continue
		}
		aggregate.messageCount++
		aggregate.lastActivityAt = laterTimePtr(aggregate.lastActivityAt, message.Timestamp)
		aggregate.lastMessageAt = laterTimePtr(aggregate.lastMessageAt, message.Timestamp)
		if preview := networkMessagePreview(message); preview != "" && aggregateMessageIsLatest(aggregate, message) {
			aggregate.lastMessagePreview = preview
		}
	}
}

func recordNetworkMessageParticipants(record func(string), message store.NetworkMessageEntry) {
	record(message.PeerFrom)
	record(message.PeerTo)
	for _, peerID := range message.Mentions {
		record(peerID)
	}
}

func (a *networkChannelAggregate) recordHistoricalParticipant(peerID string) {
	if a == nil {
		return
	}
	trimmed := strings.TrimSpace(peerID)
	if trimmed == "" {
		return
	}
	if a.historicalParticipants == nil {
		a.historicalParticipants = make(map[string]struct{})
	}
	if _, exists := a.historicalParticipants[trimmed]; exists {
		return
	}
	a.historicalParticipants[trimmed] = struct{}{}
	a.historicalParticipantCount = len(a.historicalParticipants)
}

func aggregateMessageIsLatest(
	aggregate *networkChannelAggregate,
	message store.NetworkMessageEntry,
) bool {
	return aggregate != nil &&
		aggregate.lastMessageAt != nil &&
		message.Timestamp.Equal(aggregate.lastMessageAt.UTC())
}

func statusForCreateNetworkChannelError(err error) int {
	switch {
	case errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceRootMissing):
		return StatusForWorkspaceError(err)
	case errors.Is(err, workspacepkg.ErrAgentNotAvailable):
		return StatusForSessionError(err)
	case errors.Is(err, store.ErrNetworkChannelExists):
		return http.StatusConflict
	case errors.Is(err, network.ErrInvalidField):
		return StatusForNetworkError(err)
	default:
		return http.StatusBadRequest
	}
}
