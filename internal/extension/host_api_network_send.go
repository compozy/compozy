package extensionpkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
)

func hostAPINetworkChannel(channel string) (string, error) {
	trimmed := strings.TrimSpace(channel)
	if trimmed == "" {
		return "", invalidParamsRPCError(errors.New("channel is required"))
	}
	if err := network.ValidateChannel(trimmed); err != nil {
		return "", invalidParamsRPCError(err)
	}
	return trimmed, nil
}

func hostAPINetworkSendRequestFromPayload(req apicontract.NetworkSendRequest) (network.SendRequest, error) {
	if strings.TrimSpace(req.WorkspaceID) == "" {
		return network.SendRequest{}, invalidParamsRPCError(errors.New("workspace_id is required"))
	}
	if strings.TrimSpace(req.SessionID) == "" {
		return network.SendRequest{}, invalidParamsRPCError(errors.New("session_id is required"))
	}
	if strings.TrimSpace(req.Channel) == "" {
		return network.SendRequest{}, invalidParamsRPCError(errors.New("channel is required"))
	}
	if strings.TrimSpace(req.Kind) == "" {
		return network.SendRequest{}, invalidParamsRPCError(errors.New("kind is required"))
	}
	if len(bytes.TrimSpace(req.Body)) == 0 {
		return network.SendRequest{}, invalidParamsRPCError(errors.New("body is required"))
	}
	if !json.Valid(req.Body) {
		return network.SendRequest{}, invalidParamsRPCError(errors.New("body must be valid JSON"))
	}
	if err := hostAPINetworkSendNoRawClaimToken(req); err != nil {
		return network.SendRequest{}, err
	}
	if err := hostAPINetworkSendConversation(req); err != nil {
		return network.SendRequest{}, err
	}

	sendReq := network.SendRequest{
		WorkspaceID: strings.TrimSpace(req.WorkspaceID),
		SessionID:   strings.TrimSpace(req.SessionID),
		Channel:     strings.TrimSpace(req.Channel),
		Kind:        network.Kind(strings.TrimSpace(req.Kind)),
		Body:        hostAPICloneRawMessage(req.Body),
		Mentions:    hostAPICloneTrimmedStrings(req.Mentions),
		ExpiresAt:   hostAPICloneInt64Ptr(req.ExpiresAt),
		Ext:         hostAPICloneRawMap(req.Ext),
	}
	if to := strings.TrimSpace(req.To); to != "" {
		sendReq.To = hostAPIPtrString(to)
	}
	if surface := strings.TrimSpace(req.Surface); surface != "" {
		networkSurface := network.Surface(surface)
		sendReq.Surface = &networkSurface
	}
	if threadID := strings.TrimSpace(req.ThreadID); threadID != "" {
		sendReq.ThreadID = hostAPIPtrString(threadID)
	}
	if directID := strings.TrimSpace(req.DirectID); directID != "" {
		sendReq.DirectID = hostAPIPtrString(directID)
	}
	if workID := strings.TrimSpace(req.WorkID); workID != "" {
		sendReq.WorkID = hostAPIPtrString(workID)
	}
	if replyTo := strings.TrimSpace(req.ReplyTo); replyTo != "" {
		sendReq.ReplyTo = hostAPIPtrString(replyTo)
	}
	if traceID := strings.TrimSpace(req.TraceID); traceID != "" {
		sendReq.TraceID = hostAPIPtrString(traceID)
	}
	if causationID := strings.TrimSpace(req.CausationID); causationID != "" {
		sendReq.CausationID = hostAPIPtrString(causationID)
	}
	if id := strings.TrimSpace(req.ID); id != "" {
		sendReq.ID = hostAPIPtrString(id)
	}
	return sendReq, nil
}

func hostAPINetworkSendConversation(req apicontract.NetworkSendRequest) error {
	kind := network.Kind(strings.TrimSpace(req.Kind))
	if err := kind.Validate(); err != nil {
		return invalidParamsRPCError(err)
	}

	surface := strings.TrimSpace(req.Surface)
	threadID := strings.TrimSpace(req.ThreadID)
	directID := strings.TrimSpace(req.DirectID)
	workID := strings.TrimSpace(req.WorkID)
	if kind == network.KindGreet || kind == network.KindWhois {
		if surface != "" || threadID != "" || directID != "" || workID != "" {
			return invalidParamsRPCError(fmt.Errorf(
				"%w: %s cannot carry conversation or work fields",
				network.ErrInvalidField,
				kind,
			))
		}
		return nil
	}

	if surface == "" {
		return invalidParamsRPCError(fmt.Errorf("%w: surface is required", network.ErrMissingField))
	}
	ref := network.ConversationRef{
		WorkspaceID: strings.TrimSpace(req.WorkspaceID),
		Channel:     strings.TrimSpace(req.Channel),
		Surface:     network.Surface(surface),
		ThreadID:    threadID,
		DirectID:    directID,
	}
	if err := ref.Validate(); err != nil {
		return invalidParamsRPCError(err)
	}
	if workID != "" {
		if err := network.ValidateWorkID(workID); err != nil {
			return invalidParamsRPCError(err)
		}
	}
	if kind == network.KindCapability || kind == network.KindReceipt || kind == network.KindTrace {
		if workID == "" {
			return invalidParamsRPCError(fmt.Errorf("%w: work_id is required", network.ErrMissingField))
		}
	}
	return nil
}

func hostAPINetworkSendNoRawClaimToken(req apicontract.NetworkSendRequest) error {
	payload := struct {
		Body json.RawMessage            `json:"body"`
		Ext  map[string]json.RawMessage `json:"ext,omitempty"`
	}{
		Body: req.Body,
		Ext:  req.Ext,
	}
	if err := apicontract.ValidateNoRawClaimTokenField(payload); err != nil {
		return invalidParamsRPCError(fmt.Errorf("raw claim_token fields are forbidden: %w", err))
	}
	return nil
}
