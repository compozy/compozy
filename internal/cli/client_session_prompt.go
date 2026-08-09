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

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
)

const (
	sessionPromptBodyLimit = 1 << 20
	promptResultEventName  = "prompt_result"
	promptDoneEventName    = "done"
	promptFinishEventName  = "finish"
)

var errSessionPromptBodyTooLarge = errors.New("session prompt response exceeds size limit")

var errSessionPromptStreamIncomplete = errors.New("session prompt stream ended before a terminal event")

type sessionPromptStreamState struct {
	terminal bool
	failure  error
}

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

func (c *daemonClient) doSessionPrompt(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	requestBody any,
) (record SessionPromptRecord, err error) {
	query, err = c.withFreshStreamTicket(ctx, query)
	if err != nil {
		return SessionPromptRecord{}, err
	}
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
	defer mergeResponseBodyCloseError(&err, response, method, path)

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return SessionPromptRecord{}, readSessionPromptError(response)
	}
	if sessionPromptIsEventStream(response) {
		events := make([]AgentEventRecord, 0)
		state := sessionPromptStreamState{}
		decodeErr := decodeSSE(ctx, response.Body, func(event SSEEvent) error {
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
			return state.observe(event)
		})
		record := SessionPromptRecord{Events: events}
		if decodeErr != nil || state.failure != nil {
			streamErr := errors.Join(decodeErr, state.failure)
			if c.target.isRemoteGateway() && ctx.Err() == nil {
				streamErr = newGatewayStreamInterruptedError(c.target, streamErr)
			}
			return record, streamErr
		}
		if !state.terminal {
			return record, errSessionPromptStreamIncomplete
		}
		return record, nil
	}

	body, err := readSessionPromptBody(response.Body)
	if err != nil {
		return SessionPromptRecord{}, err
	}
	var responseBody contract.SendPromptResultResponse
	if err := json.Unmarshal(body, &responseBody); err != nil {
		return SessionPromptRecord{}, fmt.Errorf("cli: decode %s %s response: %w", method, path, err)
	}
	return SessionPromptRecord{Prompt: responseBody.Prompt, Goal: responseBody.Prompt.Goal}, nil
}

func (c *daemonClient) StreamPromptSession(
	ctx context.Context,
	id string,
	request SessionPromptRequest,
	handler SSEHandler,
) (err error) {
	path, err := c.sessionScopedPath(ctx, id, "/prompt")
	if err != nil {
		return err
	}
	query, err := c.withFreshStreamTicket(ctx, nil)
	if err != nil {
		return err
	}
	response, err := c.doRequestWithCredentialsAndClient(
		ctx,
		http.MethodPost,
		path,
		query,
		request,
		"",
		agentidentity.Credentials{},
		c.streamHTTPClient(),
	)
	if err != nil {
		return err
	}
	defer mergeResponseBodyCloseError(&err, response, http.MethodPost, path)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return readSessionPromptError(response)
	}
	if sessionPromptIsEventStream(response) {
		if handler == nil {
			handler = func(SSEEvent) error { return nil }
		}
		state := sessionPromptStreamState{}
		decodeErr := decodeSSE(ctx, response.Body, func(event SSEEvent) error {
			if err := state.observe(event); err != nil {
				return err
			}
			return handler(event)
		})
		if decodeErr != nil || state.failure != nil {
			streamErr := errors.Join(decodeErr, state.failure)
			if c.target.isRemoteGateway() && ctx.Err() == nil {
				return newGatewayStreamInterruptedError(c.target, streamErr)
			}
			return streamErr
		}
		if !state.terminal {
			return errSessionPromptStreamIncomplete
		}
		return nil
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
	return handler(SSEEvent{Event: promptResultEventName, Data: body})
}

func (s *sessionPromptStreamState) observe(event SSEEvent) error {
	if strings.TrimSpace(string(event.Data)) == "[DONE]" {
		s.terminal = true
		return nil
	}
	var payload struct {
		Type      string                          `json:"type"`
		Error     string                          `json:"error"`
		ErrorText string                          `json:"errorText"`
		Failure   *contract.SessionFailurePayload `json:"failure"`
	}
	if len(event.Data) > 0 {
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			return fmt.Errorf("cli: decode prompt stream event: %w", err)
		}
	}
	eventType := strings.TrimSpace(payload.Type)
	if eventType == "" {
		eventType = strings.TrimSpace(event.Event)
	}
	switch eventType {
	case automationErrorKey:
		s.terminal = true
		message := strings.TrimSpace(payload.ErrorText)
		if message == "" {
			message = strings.TrimSpace(payload.Error)
		}
		if message == "" && payload.Failure != nil {
			message = strings.TrimSpace(payload.Failure.Summary)
		}
		if message == "" {
			message = "daemon reported a terminal prompt failure"
		}
		s.failure = fmt.Errorf("cli: session prompt stream failed: %s", message)
	case promptDoneEventName, promptFinishEventName, promptResultEventName:
		s.terminal = true
	}
	return nil
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
	payload, readErr := io.ReadAll(io.LimitReader(body, sessionPromptBodyLimit+1))
	drainErr := discardResponseBodyBounded(body)
	if drainErr != nil {
		drainErr = fmt.Errorf("cli: drain session prompt response: %w", drainErr)
	}
	if readErr != nil {
		return nil, errors.Join(fmt.Errorf("cli: read session prompt response: %w", readErr), drainErr)
	}
	if len(payload) > sessionPromptBodyLimit {
		return nil, errors.Join(
			fmt.Errorf(
				"cli: session prompt response exceeds %d bytes: %w",
				sessionPromptBodyLimit,
				errSessionPromptBodyTooLarge,
			),
			drainErr,
		)
	}
	if drainErr != nil {
		return nil, drainErr
	}
	return payload, nil
}

func decodeGoalCommandResult(body []byte) (contract.GoalCommandResult, bool) {
	var response contract.SendPromptResultResponse
	if len(body) == 0 || json.Unmarshal(body, &response) != nil || response.Prompt.Goal == nil ||
		response.Prompt.Goal.Outcome == "" {
		return contract.GoalCommandResult{}, false
	}
	return *response.Prompt.Goal, true
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
