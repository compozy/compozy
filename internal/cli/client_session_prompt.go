package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
)

const (
	sessionPromptBodyLimit = 1 << 20
	goalResultEventName    = "goal_result"
)

type goalCommandAPIError struct {
	statusCode int
	status     string
	result     contract.GoalCommandResult
}

func (e *goalCommandAPIError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	if e.result.ReasonCode != nil {
		return string(*e.result.ReasonCode)
	}
	if strings.TrimSpace(e.status) != "" {
		return fmt.Sprintf("daemon api %s: %s", e.status, e.result.Outcome)
	}
	return string(e.result.Outcome)
}

func (c *unixSocketClient) doSessionPrompt(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	requestBody any,
) (record SessionPromptRecord, err error) {
	response, err := c.doRequestWithCredentialsAndClient(
		ctx,
		method,
		path,
		query,
		requestBody,
		"",
		agentidentity.Credentials{},
		c.streamHTTPClient(),
	)
	if err != nil {
		return SessionPromptRecord{}, err
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("cli: close %s response: %w", path, closeErr))
		}
	}()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return SessionPromptRecord{}, readSessionPromptError(response)
	}
	if sessionPromptIsEventStream(response) {
		events := make([]AgentEventRecord, 0)
		if err := decodeSSE(ctx, response.Body, func(event SSEEvent) error {
			var payload AgentEventRecord
			if len(event.Data) > 0 {
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					return fmt.Errorf("cli: decode prompt event: %w", err)
				}
			}
			if payload.Type == "" {
				payload.Type = event.Event
			}
			events = append(events, payload)
			return nil
		}); err != nil {
			return SessionPromptRecord{}, err
		}
		return SessionPromptRecord{Events: events}, nil
	}

	body, err := readSessionPromptBody(response.Body)
	if err != nil {
		return SessionPromptRecord{}, err
	}
	if goalResult, ok := decodeGoalCommandResult(body); ok {
		return SessionPromptRecord{Goal: &goalResult}, nil
	}
	var responseBody contract.SendPromptResultResponse
	if err := json.Unmarshal(body, &responseBody); err != nil {
		return SessionPromptRecord{}, fmt.Errorf("cli: decode %s %s response: %w", method, path, err)
	}
	return SessionPromptRecord{Prompt: responseBody.Prompt}, nil
}

func (c *unixSocketClient) StreamPromptSession(
	ctx context.Context,
	id string,
	message string,
	handler SSEHandler,
) (err error) {
	path, err := c.sessionScopedPath(ctx, id, "/prompt")
	if err != nil {
		return err
	}
	response, err := c.doRequestWithCredentialsAndClient(
		ctx,
		http.MethodPost,
		path,
		nil,
		map[string]string{clientMessageKey: message},
		"",
		agentidentity.Credentials{},
		c.streamHTTPClient(),
	)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("cli: close %s response: %w", path, closeErr))
		}
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return readSessionPromptError(response)
	}
	if sessionPromptIsEventStream(response) {
		if handler == nil {
			return drainResponseBody(http.MethodPost, path, response.Body)
		}
		return decodeSSE(ctx, response.Body, handler)
	}
	body, err := readSessionPromptBody(response.Body)
	if err != nil {
		return err
	}
	if !json.Valid(body) {
		return fmt.Errorf("cli: decode POST %s response: invalid JSON", path)
	}
	if handler == nil {
		return nil
	}
	return handler(SSEEvent{Event: goalResultEventName, Data: body})
}

func sessionPromptIsEventStream(response *http.Response) bool {
	if response == nil {
		return false
	}
	return strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream")
}

func readSessionPromptError(response *http.Response) error {
	body, err := readSessionPromptBody(response.Body)
	if err != nil {
		return err
	}
	if goalResult, ok := decodeGoalCommandResult(body); ok {
		return &goalCommandAPIError{
			statusCode: response.StatusCode,
			status:     response.Status,
			result:     goalResult,
		}
	}
	return readAPIErrorBody(response.StatusCode, response.Status, body)
}

func readSessionPromptBody(body io.Reader) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(body, sessionPromptBodyLimit))
	if err != nil {
		return nil, fmt.Errorf("cli: read session prompt response: %w", err)
	}
	return payload, nil
}

func decodeGoalCommandResult(body []byte) (contract.GoalCommandResult, bool) {
	var result contract.GoalCommandResult
	if len(body) == 0 || json.Unmarshal(body, &result) != nil || result.Outcome == "" {
		return contract.GoalCommandResult{}, false
	}
	return result, true
}

func marshalGoalCommandExecutionError(args []string, goalErr *goalCommandAPIError) ([]byte, bool) {
	if goalErr == nil {
		return nil, false
	}
	switch requestedOutputFormat(args) {
	case OutputJSON:
		payload, err := json.Marshal(goalErr.result)
		return payload, err == nil
	case OutputJSONL:
		payload, err := json.Marshal(goalErr.result)
		if err != nil {
			return nil, false
		}
		return append(payload, '\n'), true
	default:
		return nil, false
	}
}
