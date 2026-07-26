package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"

	"github.com/compozy/agh/internal/network"

	"github.com/compozy/agh/internal/store"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) networkPeers(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkPeersInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	peers, err := n.deps.Network.ListPeers(ctx, workspaceID, input.Channel)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredNetworkResult(map[string]any{"peers": peers}, fmt.Sprintf("%d peers", len(peers)))
}

func (n *daemonNativeTools) networkStatus(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct{}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	status, err := n.deps.Network.Status(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.NetworkStatusPayloadFromStatus(status)
	if payload == nil {
		return toolspkg.ToolResult{}, errors.New("daemon: network status is required")
	}
	return structuredNetworkResult(map[string]any{nativeToolsNetworkKey: payload}, payload.Status)
}

func (n *daemonNativeTools) networkChannels(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkChannelsInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	bound, err := n.nativeBoundSession(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(
		ctx,
		req.ToolID,
		nativeBoundWorkspaceRef(bound, input.WorkspaceID),
		scope,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	var channels any
	var count int
	if n.deps.Sessions != nil && n.deps.NetworkStore != nil {
		payload, err := core.NetworkChannelPayloads(
			ctx,
			n.deps.Network,
			n.deps.Sessions,
			n.deps.NetworkStore,
			workspaceID,
		)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		payload = nativeFilterNetworkChannelPayloads(bound, payload)
		channels = payload
		count = len(payload)
	} else {
		infos, err := n.deps.Network.ListChannels(ctx, workspaceID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		payload := core.NetworkChannelPayloadsFromInfos(infos)
		payload = nativeFilterNetworkChannelPayloads(bound, payload)
		channels = payload
		count = len(payload)
	}
	return structuredNetworkResult(map[string]any{"channels": channels}, fmt.Sprintf("%d channels", count))
}

func (n *daemonNativeTools) networkInbox(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkInboxInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	bound, err := n.nativeBoundSession(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID := nativeBoundSessionID(bound, input.SessionID, req.SessionID, scope.SessionID)
	if sessionID == "" {
		return toolspkg.ToolResult{}, nativeRequiredInputError(req.ToolID, "session_id")
	}
	resolved, err := n.nativeResolvedWorkspace(
		ctx,
		req.ToolID,
		nativeBoundWorkspaceRef(bound, input.WorkspaceID),
		scope,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionWorkspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	if err := n.requireNativeSessionWorkspace(ctx, req.ToolID, sessionWorkspaceID, sessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	messages, err := n.deps.Network.Inbox(ctx, sessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	messages = nativeFilterNetworkEnvelopes(bound, messages)
	payload := core.NetworkEnvelopePayloadsFromEnvelopes(messages)
	return structuredNetworkResult(
		map[string]any{nativeToolsMessagesKey: payload},
		fmt.Sprintf("%d messages", len(payload)),
	)
}

func (n *daemonNativeTools) networkSend(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkSendInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	bound, err := n.nativeBoundSession(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	channel := strings.TrimSpace(input.Channel)
	if !nativeBoundSessionAllowsChannel(bound, channel) {
		return toolspkg.ToolResult{}, nativeBoundChannelDenied(req.ToolID, channel)
	}
	sessionID := nativeBoundSessionID(bound, input.SessionID, req.SessionID, scope.SessionID)
	resolved, err := n.nativeResolvedWorkspace(
		ctx,
		req.ToolID,
		nativeBoundWorkspaceRef(bound, input.WorkspaceID),
		scope,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	if err := n.requireNativeSessionWorkspace(ctx, req.ToolID, workspaceID, sessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sendReq, err := core.NetworkSendRequestFromPayload(contract.NetworkSendRequest{
		WorkspaceID: workspaceID,
		SessionID:   sessionID,
		Channel:     channel,
		Surface:     strings.TrimSpace(input.Surface),
		ThreadID:    strings.TrimSpace(input.ThreadID),
		DirectID:    strings.TrimSpace(input.DirectID),
		Kind:        strings.TrimSpace(input.Kind),
		To:          strings.TrimSpace(input.To),
		Mentions:    cloneTrimmedStrings(input.Mentions),
		Body:        cloneJSON(input.Body),
		WorkID:      strings.TrimSpace(input.WorkID),
		ReplyTo:     strings.TrimSpace(input.ReplyTo),
		TraceID:     strings.TrimSpace(input.TraceID),
		CausationID: strings.TrimSpace(input.CausationID),
		ExpiresAt:   input.ExpiresAt,
		ID:          strings.TrimSpace(input.ID),
		Ext:         map[string]json.RawMessage(cloneExtensionMap(input.Ext)),
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkSendToolError(req.ToolID, err)
	}
	messageID, err := n.deps.Network.Send(ctx, sendReq)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredNetworkResult(map[string]any{"message_id": messageID}, messageID)
}

func (n *daemonNativeTools) networkDirectResolve(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkDirectResolveInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	channel, err := nativeNetworkChannel(req.ToolID, input.Channel)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID := firstNonEmpty(input.SessionID, req.SessionID, scope.SessionID)
	if sessionID == "" {
		return toolspkg.ToolResult{}, nativeRequiredInputError(req.ToolID, "session_id")
	}
	peerID := strings.TrimSpace(input.PeerID)
	if err := network.ValidatePeerID(peerID); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	localPeer, remotePeer, err := n.resolveNetworkDirectRoomPeers(ctx, workspaceID, channel, sessionID, peerID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkToolError(req.ToolID, err)
	}
	if strings.TrimSpace(*localPeer.SessionID) == strings.TrimSpace(*remotePeer.SessionID) {
		return toolspkg.ToolResult{}, nativeNetworkInputError(
			req.ToolID,
			fmt.Errorf("%w: direct room sessions must differ", network.ErrInvalidField),
		)
	}
	directID, sessionA, sessionB, err := store.NetworkDirectRoomIdentity(
		workspaceID,
		channel,
		*localPeer.SessionID,
		*remotePeer.SessionID,
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkToolError(req.ToolID, err)
	}
	now := time.Now().UTC()
	direct, err := n.deps.NetworkStore.ResolveDirectRoom(ctx, store.NetworkDirectRoomEntry{
		WorkspaceID:    workspaceID,
		Channel:        channel,
		DirectID:       directID,
		SessionA:       sessionA,
		SessionB:       sessionB,
		OpenedAt:       now,
		LastActivityAt: now,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.NetworkDirectRoomPayloadFromStore(direct)
	return structuredNetworkResult(map[string]any{nativeToolsDirectKey: payload}, payload.DirectID)
}

func (n *daemonNativeTools) networkWork(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkWorkInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workID := strings.TrimSpace(input.WorkID)
	if err := network.ValidateWorkID(workID); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	work, err := n.deps.NetworkStore.GetWork(ctx, workspaceID, workID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.NetworkWorkPayloadFromStore(work)
	return structuredNetworkResult(map[string]any{"work": payload}, payload.WorkID)
}

func (n *daemonNativeTools) resolveNetworkDirectRoomPeers(
	ctx context.Context,
	workspaceID string,
	channel string,
	sessionID string,
	peerID string,
) (network.PeerInfo, network.PeerInfo, error) {
	peers, err := n.deps.Network.ListPeers(ctx, workspaceID, channel)
	if err != nil {
		return network.PeerInfo{}, network.PeerInfo{}, err
	}
	var local network.PeerInfo
	localFound := false
	var remote network.PeerInfo
	remoteFound := false
	for _, peer := range peers {
		if strings.TrimSpace(peer.PeerID) == peerID {
			remote = peer
			remoteFound = true
		}
		if peer.SessionID == nil || strings.TrimSpace(*peer.SessionID) != sessionID || !peer.Local {
			continue
		}
		local = peer
		localFound = true
	}
	if !localFound {
		return network.PeerInfo{}, network.PeerInfo{}, fmt.Errorf(
			"%w: session=%q channel=%q",
			network.ErrLocalPeerNotFound,
			sessionID,
			channel,
		)
	}
	if !remoteFound || remote.SessionID == nil || strings.TrimSpace(*remote.SessionID) == "" {
		return network.PeerInfo{}, network.PeerInfo{}, fmt.Errorf(
			"%w: peer_id=%q channel=%q",
			network.ErrTargetPeerNotFound,
			peerID,
			channel,
		)
	}
	return local, remote, nil
}
