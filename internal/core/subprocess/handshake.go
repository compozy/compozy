package subprocess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// InitializeRequest describes the protocol version and capability contract offered by the host.
type InitializeRequest struct {
	ProtocolVersion           string   `json:"protocol_version"`
	SupportedProtocolVersions []string `json:"supported_protocol_versions,omitempty"`
	GrantedCapabilities       []string `json:"granted_capabilities,omitempty"`
}

// InitializeResponse describes the negotiated protocol version and accepted capability contract.
type InitializeResponse struct {
	ProtocolVersion      string   `json:"protocol_version"`
	AcceptedCapabilities []string `json:"accepted_capabilities,omitempty"`
}

// InitializeCanceledError reports context cancellation during the initialize handshake.
type InitializeCanceledError struct {
	Cause error
}

func (e *InitializeCanceledError) Error() string {
	if e == nil || e.Cause == nil {
		return "initialize subprocess canceled"
	}
	return fmt.Sprintf("initialize subprocess canceled: %v", e.Cause)
}

func (e *InitializeCanceledError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Initialize sends an initialize request and validates the selected protocol version.
func Initialize(
	ctx context.Context,
	transport *Transport,
	requestID json.RawMessage,
	request InitializeRequest,
) (InitializeResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if transport == nil {
		return InitializeResponse{}, fmt.Errorf("initialize subprocess transport: missing transport")
	}
	messageID, err := writeInitializeRequest(transport, requestID, request)
	if err != nil {
		return InitializeResponse{}, err
	}
	return waitForInitializeResponse(ctx, transport, messageID, request)
}

type initializeReadResult struct {
	message Message
	err     error
}

func writeInitializeRequest(
	transport *Transport,
	requestID json.RawMessage,
	request InitializeRequest,
) (json.RawMessage, error) {
	if len(requestID) == 0 {
		requestID = json.RawMessage("1")
	}

	params, err := json.Marshal(request)
	if err != nil {
		return nil, NewInvalidParams(map[string]any{"error": err.Error()})
	}
	messageID := json.RawMessage(append([]byte(nil), requestID...))
	if err := transport.WriteMessage(Message{
		ID:     &messageID,
		Method: "initialize",
		Params: params,
	}); err != nil {
		return nil, NewInternalError(map[string]any{"error": err.Error()})
	}
	return messageID, nil
}

func waitForInitializeResponse(
	ctx context.Context,
	transport *Transport,
	messageID json.RawMessage,
	request InitializeRequest,
) (InitializeResponse, error) {
	readResult := make(chan initializeReadResult, 1)
	go func() {
		message, readErr := transport.ReadMessage()
		readResult <- initializeReadResult{message: message, err: readErr}
	}()

	select {
	case <-ctx.Done():
		return InitializeResponse{}, initializeCanceled(ctx, transport)
	case response := <-readResult:
		return decodeInitializeResponse(response, messageID, request)
	}
}

func initializeCanceled(ctx context.Context, transport *Transport) error {
	cancelErr := context.Cause(ctx)
	if cancelErr == nil {
		cancelErr = ctx.Err()
	}
	err := &InitializeCanceledError{Cause: cancelErr}
	if closeErr := transport.Close(); closeErr != nil {
		return errors.Join(err, fmt.Errorf("close initialize transport: %w", closeErr))
	}
	return err
}

func decodeInitializeResponse(
	response initializeReadResult,
	messageID json.RawMessage,
	request InitializeRequest,
) (InitializeResponse, error) {
	if response.err != nil {
		return InitializeResponse{}, response.err
	}
	if response.message.Error != nil {
		return InitializeResponse{}, response.message.Error
	}
	if response.message.ID == nil || string(*response.message.ID) != string(messageID) {
		return InitializeResponse{}, NewInvalidRequest(
			map[string]any{"reason": "unexpected_initialize_response_id"},
		)
	}

	var initializeResponse InitializeResponse
	if err := json.Unmarshal(response.message.Result, &initializeResponse); err != nil {
		return InitializeResponse{}, NewInternalError(map[string]any{"error": err.Error()})
	}
	if err := ValidateInitializeResponse(request, initializeResponse); err != nil {
		return InitializeResponse{}, err
	}
	return initializeResponse, nil
}

// ValidateInitializeResponse checks the negotiated protocol version and capability subset.
func ValidateInitializeResponse(request InitializeRequest, response InitializeResponse) error {
	if !containsString(request.SupportedProtocolVersions, response.ProtocolVersion) {
		return NewInvalidParams(map[string]any{
			"reason":                      "unsupported_protocol_version",
			"requested":                   response.ProtocolVersion,
			"supported_protocol_versions": request.SupportedProtocolVersions,
		})
	}
	if !isSubset(response.AcceptedCapabilities, request.GrantedCapabilities) {
		return NewInvalidParams(map[string]any{
			"reason":                "unsupported_capability_acceptance",
			"accepted_capabilities": response.AcceptedCapabilities,
			"granted_capabilities":  request.GrantedCapabilities,
		})
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isSubset(values []string, allowed []string) bool {
	for _, value := range values {
		if !containsString(allowed, value) {
			return false
		}
	}
	return true
}
