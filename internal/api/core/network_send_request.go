package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
	toolspkg "github.com/compozy/agh/internal/tools"
)

// NetworkSendRequestFromPayload validates and converts one shared send payload into the runtime request.
func NetworkSendRequestFromPayload(req contract.NetworkSendRequest) (network.SendRequest, error) {
	if strings.TrimSpace(req.SessionID) == "" {
		return network.SendRequest{}, NewNetworkValidationError(errors.New("session_id is required"))
	}
	if strings.TrimSpace(req.WorkspaceID) == "" {
		return network.SendRequest{}, NewNetworkValidationError(errors.New("workspace_id is required"))
	}
	if strings.TrimSpace(req.Channel) == "" {
		return network.SendRequest{}, NewNetworkValidationError(errors.New("channel is required"))
	}
	if strings.TrimSpace(req.Kind) == "" {
		return network.SendRequest{}, NewNetworkValidationError(errors.New("kind is required"))
	}
	if len(bytes.TrimSpace(req.Body)) == 0 {
		return network.SendRequest{}, NewNetworkValidationError(errors.New("body is required"))
	}
	if !json.Valid(req.Body) {
		return network.SendRequest{}, NewNetworkValidationError(errors.New("body must be valid JSON"))
	}
	if err := validateNetworkSendNoRawClaimToken(req); err != nil {
		return network.SendRequest{}, err
	}
	if err := validateNetworkSendConversation(req); err != nil {
		return network.SendRequest{}, err
	}

	return networkSendRequestFromValidatedPayload(req), nil
}

func networkSendRequestFromValidatedPayload(req contract.NetworkSendRequest) network.SendRequest {
	sendReq := network.SendRequest{
		SessionID:   strings.TrimSpace(req.SessionID),
		WorkspaceID: strings.TrimSpace(req.WorkspaceID),
		Channel:     strings.TrimSpace(req.Channel),
		Kind:        network.Kind(strings.TrimSpace(req.Kind)),
		Body:        cloneRawMessage(req.Body),
		Mentions:    cloneTrimmedStrings(req.Mentions),
		ExpiresAt:   cloneInt64Ptr(req.ExpiresAt),
		Ext:         cloneRawMap(req.Ext),
	}
	if to := strings.TrimSpace(req.To); to != "" {
		sendReq.To = ptrString(to)
	}
	if surface := strings.TrimSpace(req.Surface); surface != "" {
		networkSurface := network.Surface(surface)
		sendReq.Surface = &networkSurface
	}
	if threadID := strings.TrimSpace(req.ThreadID); threadID != "" {
		sendReq.ThreadID = ptrString(threadID)
	}
	if directID := strings.TrimSpace(req.DirectID); directID != "" {
		sendReq.DirectID = ptrString(directID)
	}
	if workID := strings.TrimSpace(req.WorkID); workID != "" {
		sendReq.WorkID = ptrString(workID)
	}
	if replyTo := strings.TrimSpace(req.ReplyTo); replyTo != "" {
		sendReq.ReplyTo = ptrString(replyTo)
	}
	if traceID := strings.TrimSpace(req.TraceID); traceID != "" {
		sendReq.TraceID = ptrString(traceID)
	}
	if causationID := strings.TrimSpace(req.CausationID); causationID != "" {
		sendReq.CausationID = ptrString(causationID)
	}
	if id := strings.TrimSpace(req.ID); id != "" {
		sendReq.ID = ptrString(id)
	}
	return sendReq
}

func validateNetworkSendConversation(req contract.NetworkSendRequest) error {
	kind := network.Kind(strings.TrimSpace(req.Kind))
	if err := kind.Validate(); err != nil {
		return NewNetworkValidationError(err)
	}

	surface := strings.TrimSpace(req.Surface)
	threadID := strings.TrimSpace(req.ThreadID)
	directID := strings.TrimSpace(req.DirectID)
	workID := strings.TrimSpace(req.WorkID)
	target := strings.TrimSpace(req.To)
	if isNetworkDiscoveryKind(kind) {
		return validateNetworkSendDiscoveryFields(kind, surface, threadID, directID, workID)
	}
	if err := validateNetworkSendConversationRef(req, surface, threadID, directID); err != nil {
		return err
	}
	return validateNetworkSendLifecycleFields(kind, workID, target)
}

func isNetworkDiscoveryKind(kind network.Kind) bool {
	return kind == network.KindGreet || kind == network.KindWhois
}

func validateNetworkSendDiscoveryFields(
	kind network.Kind,
	surface string,
	threadID string,
	directID string,
	workID string,
) error {
	if surface == "" && threadID == "" && directID == "" && workID == "" {
		return nil
	}
	return NewNetworkValidationError(fmt.Errorf(
		"%w: %s cannot carry conversation or work fields",
		network.ErrInvalidField,
		kind,
	))
}

func validateNetworkSendConversationRef(
	req contract.NetworkSendRequest,
	surface string,
	threadID string,
	directID string,
) error {
	if surface == "" {
		return NewNetworkValidationError(fmt.Errorf("%w: surface is required", network.ErrMissingField))
	}
	ref := network.ConversationRef{
		WorkspaceID: strings.TrimSpace(req.WorkspaceID),
		Channel:     strings.TrimSpace(req.Channel),
		Surface:     network.Surface(surface),
		ThreadID:    threadID,
		DirectID:    directID,
	}
	if err := ref.Validate(); err != nil {
		return NewNetworkValidationError(err)
	}
	return nil
}

func validateNetworkSendLifecycleFields(kind network.Kind, workID string, target string) error {
	if workID != "" {
		if err := network.ValidateWorkID(workID); err != nil {
			return NewNetworkValidationError(err)
		}
	}
	if networkSendKindRequiresWork(kind) && workID == "" {
		return NewNetworkValidationError(fmt.Errorf("%w: work_id is required", network.ErrMissingField))
	}
	if kind == network.KindCapability && target == "" {
		return NewNetworkValidationError(fmt.Errorf("%w: capability to is required", network.ErrMissingField))
	}
	if kind == network.KindSay && workID != "" && target == "" {
		return NewNetworkValidationError(fmt.Errorf("%w: lifecycle say to is required", network.ErrMissingField))
	}
	return nil
}

func networkSendKindRequiresWork(kind network.Kind) bool {
	return kind == network.KindCapability || kind == network.KindReceipt || kind == network.KindTrace
}

// validateNetworkSendNoRawClaimToken keeps raw claim_token fields out of client-controlled network payloads.
func validateNetworkSendNoRawClaimToken(req contract.NetworkSendRequest) error {
	payload := struct {
		Body json.RawMessage            `json:"body"`
		Ext  map[string]json.RawMessage `json:"ext,omitempty"`
	}{
		Body: req.Body,
		Ext:  req.Ext,
	}
	if err := contract.ValidateNoRawClaimTokenField(payload); err != nil {
		return NewNetworkValidationError(fmt.Errorf(
			"%s: raw claim_token fields are forbidden: %w",
			toolspkg.ReasonNetworkRawTokenRejected,
			err,
		))
	}
	return nil
}
