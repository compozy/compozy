package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

type sourceCoordinationMetadata struct {
	metadata contract.CoordinationMessageMetadataPayload
	ok       bool
}

type decodedAgentChannelReplyRequest struct {
	ReplyToMessageID string
	Body             json.RawMessage
	Metadata         contract.CoordinationMessageMetadataPayload
	IdempotencyKey   string
}

type agentChannelReplyTarget struct {
	peerID    string
	sessionID string
}

// AgentChannelReply replies to one queued or persisted message using the validated caller identity.
func (h *BaseHandlers) AgentChannelReply(c *gin.Context) {
	caller, ok := h.requireAgentCaller(c, agentActionChannelReply)
	if !ok {
		return
	}
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}

	req, err := decodeAgentChannelReplyRequest(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	source, sourceMetadata, err := h.resolveAgentReplySource(
		c.Request.Context(),
		service,
		caller,
		req.ReplyToMessageID,
	)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	target, err := resolveAgentReplyTarget(c.Request.Context(), service, source)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	metadata, err := agentChannelReplyMetadata(req.Metadata, sourceMetadata)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if err := validateAgentChannelRequest(req.Body, metadata, struct {
		Body     json.RawMessage                             `json:"body"`
		Metadata contract.CoordinationMessageMetadataPayload `json:"metadata"`
	}{Body: req.Body, Metadata: metadata}); err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}

	ext, err := coordinationExt(metadata)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	sendReq := agentChannelReplySendRequest(caller, source, target.peerID, req, ext)
	if idempotencyKey := strings.TrimSpace(req.IdempotencyKey); idempotencyKey != "" {
		sendReq.ID = ptrString(idempotencyKey)
	}

	messageID, err := service.Send(c.Request.Context(), sendReq)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusAccepted, contract.AgentChannelMessageResponse{
		Message: agentChannelMessageFromRequest(
			messageID,
			source.Channel,
			caller.Session.ID,
			target.sessionID,
			req.Body,
			metadata,
			h.nowUTC(),
		),
	})
}

func agentChannelReplyMetadata(
	requestMetadata contract.CoordinationMessageMetadataPayload,
	sourceMetadata sourceCoordinationMetadata,
) (contract.CoordinationMessageMetadataPayload, error) {
	if zeroCoordinationMetadata(requestMetadata) {
		if !sourceMetadata.ok {
			return contract.CoordinationMessageMetadataPayload{}, NewNetworkValidationError(errors.New(
				"metadata is required when the source message has no coordination metadata",
			))
		}
		metadata := sourceMetadata.metadata
		metadata.MessageKind = contract.CoordinationMessageReply
		return metadata, nil
	}
	if requestMetadata.MessageKind != contract.CoordinationMessageReply {
		return contract.CoordinationMessageMetadataPayload{}, NewNetworkValidationError(errors.New(
			"metadata.message_kind must be reply for agent channel replies",
		))
	}
	return requestMetadata, nil
}

func agentChannelReplySendRequest(
	caller agentidentity.Caller,
	source network.Envelope,
	targetPeerID string,
	req decodedAgentChannelReplyRequest,
	ext map[string]json.RawMessage,
) network.SendRequest {
	sendReq := network.SendRequest{
		SessionID:   strings.TrimSpace(caller.Session.ID),
		WorkspaceID: strings.TrimSpace(caller.Session.WorkspaceID),
		Channel:     strings.TrimSpace(source.Channel),
		Surface:     new(network.SurfaceDirect),
		Kind:        network.KindSay,
		To:          ptrString(targetPeerID),
		ReplyTo:     ptrString(req.ReplyToMessageID),
		Body:        cloneRawMessage(req.Body),
		Ext:         ext,
	}
	if source.WorkID != nil && strings.TrimSpace(*source.WorkID) != "" {
		sendReq.WorkID = ptrString(*source.WorkID)
	}
	if source.TraceID != nil && strings.TrimSpace(*source.TraceID) != "" {
		sendReq.TraceID = ptrString(*source.TraceID)
	}
	return sendReq
}

func (h *BaseHandlers) resolveAgentReplySource(
	ctx context.Context,
	service NetworkService,
	caller agentidentity.Caller,
	messageID string,
) (network.Envelope, sourceCoordinationMetadata, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return network.Envelope{}, sourceCoordinationMetadata{}, NewNetworkValidationError(
			errors.New("reply_to_message_id is required"),
		)
	}

	callerSessionID := strings.TrimSpace(caller.Session.ID)
	callerWorkspaceID := strings.TrimSpace(caller.Session.WorkspaceID)
	envelopes, err := service.Inbox(ctx, callerSessionID)
	if err != nil {
		return network.Envelope{}, sourceCoordinationMetadata{}, err
	}
	for _, envelope := range envelopes {
		if strings.TrimSpace(envelope.ID) != messageID {
			continue
		}
		metadata, ok := coordinationMetadataFromEnvelope(envelope)
		return envelope, sourceCoordinationMetadata{metadata: metadata, ok: ok}, validateReplySource(
			envelope,
			callerWorkspaceID,
		)
	}

	if h != nil && h.NetworkStore != nil {
		entries, lookupErr := h.NetworkStore.ListNetworkMessages(ctx, store.NetworkMessageQuery{
			WorkspaceID: callerWorkspaceID,
			SessionID:   callerSessionID,
			MessageID:   messageID,
			Limit:       1,
		})
		if lookupErr != nil {
			return network.Envelope{}, sourceCoordinationMetadata{}, lookupErr
		}
		if len(entries) > 0 {
			envelope, convErr := envelopeFromNetworkMessage(entries[0])
			if convErr != nil {
				return network.Envelope{}, sourceCoordinationMetadata{}, convErr
			}
			metadata, ok := coordinationMetadataFromEnvelope(envelope)
			return envelope, sourceCoordinationMetadata{metadata: metadata, ok: ok}, validateReplySource(
				envelope,
				callerWorkspaceID,
			)
		}
	}

	return network.Envelope{}, sourceCoordinationMetadata{}, fmt.Errorf(
		"%w: message_id=%q",
		network.ErrTargetPeerNotFound,
		messageID,
	)
}

func validateReplySource(envelope network.Envelope, callerWorkspaceID string) error {
	workspaceID := strings.TrimSpace(envelope.WorkspaceID)
	if workspaceID == "" {
		return NewNetworkValidationError(errors.New("source message workspace_id is required"))
	}
	if workspaceID != strings.TrimSpace(callerWorkspaceID) {
		return NewNetworkValidationError(errors.New("source message workspace does not match caller workspace"))
	}
	if strings.TrimSpace(envelope.Channel) == "" {
		return NewNetworkValidationError(errors.New("source message channel is required"))
	}
	if strings.TrimSpace(envelope.From) == "" {
		return NewNetworkValidationError(errors.New("source message sender is required"))
	}
	return nil
}

func resolveAgentReplyTarget(
	ctx context.Context,
	service NetworkService,
	source network.Envelope,
) (agentChannelReplyTarget, error) {
	workspaceID := strings.TrimSpace(source.WorkspaceID)
	channel := strings.TrimSpace(source.Channel)
	sourceIdentity := strings.TrimSpace(source.From)
	peers, err := service.ListPeers(ctx, workspaceID, channel)
	if err != nil {
		return agentChannelReplyTarget{}, err
	}
	for _, peer := range peers {
		peerID := strings.TrimSpace(peer.PeerID)
		sessionID := stringPtrValue(peer.SessionID)
		if peerID == "" || sessionID == "" {
			continue
		}
		if sourceIdentity == peerID || sourceIdentity == sessionID {
			return agentChannelReplyTarget{peerID: peerID, sessionID: sessionID}, nil
		}
	}
	return agentChannelReplyTarget{}, fmt.Errorf(
		"%w: reply source %q is not active in %s/%s",
		network.ErrTargetPeerNotFound,
		sourceIdentity,
		workspaceID,
		channel,
	)
}

func decodeAgentChannelReplyRequest(c *gin.Context) (decodedAgentChannelReplyRequest, error) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return decodedAgentChannelReplyRequest{}, NewNetworkValidationError(
			errors.New("reply request body is required"),
		)
	}
	var raw map[string]json.RawMessage
	if err := decodeStrictJSONBody(c, &raw); err != nil {
		return decodedAgentChannelReplyRequest{}, fmt.Errorf("decode agent channel reply request: %w", err)
	}
	if err := validateAgentChannelReplyRequestFields(raw); err != nil {
		return decodedAgentChannelReplyRequest{}, NewNetworkValidationError(err)
	}
	if err := contract.ValidateNoRawClaimTokenField(raw); err != nil {
		return decodedAgentChannelReplyRequest{}, NewNetworkValidationError(err)
	}

	var req decodedAgentChannelReplyRequest
	if err := decodeRawString(raw["reply_to_message_id"], &req.ReplyToMessageID); err != nil {
		return decodedAgentChannelReplyRequest{}, NewNetworkValidationError(fmt.Errorf("reply_to_message_id: %w", err))
	}
	if err := decodeRawString(raw["idempotency_key"], &req.IdempotencyKey); err != nil {
		return decodedAgentChannelReplyRequest{}, NewNetworkValidationError(fmt.Errorf("idempotency_key: %w", err))
	}
	req.Body = cloneRawMessage(raw["body"])
	if len(bytes.TrimSpace(req.Body)) == 0 {
		return decodedAgentChannelReplyRequest{}, NewNetworkValidationError(errors.New("body is required"))
	}

	if metadataRaw := bytes.TrimSpace(raw["metadata"]); len(metadataRaw) > 0 &&
		!bytes.Equal(metadataRaw, []byte("null")) &&
		!bytes.Equal(metadataRaw, []byte("{}")) {
		metadata, ok, err := decodeOptionalCoordinationMetadata(metadataRaw)
		if err != nil {
			return decodedAgentChannelReplyRequest{}, NewNetworkValidationError(fmt.Errorf("metadata: %w", err))
		}
		if ok {
			req.Metadata = metadata
		}
	}
	return req, nil
}

func validateAgentChannelReplyRequestFields(raw map[string]json.RawMessage) error {
	if err := contract.ValidateNoCallerSuppliedNetworkIdentity(raw, "agent channel reply request"); err != nil {
		return err
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		switch key {
		case "reply_to_message_id", "body", "metadata", "idempotency_key":
			continue
		default:
			return fmt.Errorf("%w: %s", ErrUnknownJSONField, key)
		}
	}
	return nil
}

func decodeOptionalCoordinationMetadata(
	raw json.RawMessage,
) (contract.CoordinationMessageMetadataPayload, bool, error) {
	type metadataAlias contract.CoordinationMessageMetadataPayload
	var decoded metadataAlias
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return contract.CoordinationMessageMetadataPayload{}, false, normalizeStrictJSONDecodeError(err)
	}
	metadata := contract.CoordinationMessageMetadataPayload(decoded)
	if zeroCoordinationMetadata(metadata) {
		return contract.CoordinationMessageMetadataPayload{}, false, nil
	}
	if err := metadata.Validate(); err != nil {
		return contract.CoordinationMessageMetadataPayload{}, false, err
	}
	return metadata, true, nil
}

func decodeRawString(raw json.RawMessage, target *string) error {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	*target = strings.TrimSpace(value)
	return nil
}
