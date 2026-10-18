package client

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// PromptInput sends input to a session, optionally selecting a busy-input mode.
type PromptInput struct {
	WorkspaceID, SessionID, Message, Mode, IdempotencyKey, ExpectedTurnID string
}

// PromptChunk retains a complete AI SDK UI message stream chunk.
type PromptChunk struct {
	Type string
	Raw  json.RawMessage
}

// TextDelta returns the text carried by a text-delta chunk, or an empty string.
func (p PromptChunk) TextDelta() string {
	if p.Type != "text-delta" {
		return ""
	}
	var chunk struct {
		Delta string `json:"delta"`
	}
	if err := json.Unmarshal(p.Raw, &chunk); err != nil {
		return ""
	}
	return chunk.Delta
}

// PromptStream represents either accepted input or a streaming prompt response.
type PromptStream struct {
	accepted *SendPromptResultPayload
	stream   *Stream[json.RawMessage]
	finished bool
}

var _ io.Closer = (*PromptStream)(nil)

// Prompt submits one turn and never reconnects or resubmits an interrupted stream.
func (c *Client) Prompt(ctx context.Context, in PromptInput) (*PromptStream, error) {
	if strings.TrimSpace(in.WorkspaceID) == "" || strings.TrimSpace(in.SessionID) == "" {
		return nil, errors.New("compozy client: prompt requires workspace and session IDs")
	}
	key := in.IdempotencyKey
	if key == "" {
		key = rand.Text()
	}
	// Derive the same message identity when an app retries an idempotency key.
	digest := sha256.Sum256([]byte(key))
	body := struct {
		Message        string `json:"message"`
		MessageID      string `json:"message_id"`
		Mode           string `json:"mode,omitempty"`
		IdempotencyKey string `json:"idempotency_key"`
		ExpectedTurnID string `json:"expected_turn_id,omitempty"`
	}{in.Message, "msg_" + hex.EncodeToString(digest[:16]), in.Mode, key, in.ExpectedTurnID}
	path := "/api/workspaces/" + url.PathEscape(in.WorkspaceID) + "/sessions/" + url.PathEscape(in.SessionID) + "/prompt"
	response, err := c.request(ctx, http.MethodPost, path, http.Header{"Accept": {"text/event-stream, application/json"}}, body, "application/json")
	if err != nil {
		return nil, &StreamError{Op: "sendSessionPrompt", Err: err}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.Join(responseError(response), drainClose(response.Body))
	}
	if isEventStream(response) {
		return &PromptStream{stream: &Stream[json.RawMessage]{decoder: newSSEDecoder(ctx, response.Body), op: "sendSessionPrompt"}}, nil
	}
	var result struct {
		Prompt *SendPromptResultPayload `json:"prompt"`
	}
	decodeErr := json.NewDecoder(response.Body).Decode(&result)
	err = errors.Join(decodeErr, drainClose(response.Body))
	if err != nil {
		return nil, fmt.Errorf("compozy client: decode accepted prompt: %w", err)
	}
	if result.Prompt == nil {
		return nil, errors.New("compozy client: accepted prompt response has no prompt payload")
	}
	return &PromptStream{accepted: result.Prompt}, nil
}

// Accepted returns the non-streaming prompt result when input was accepted.
func (s *PromptStream) Accepted() (*SendPromptResultPayload, bool) {
	return s.accepted, s.accepted != nil
}

// Next returns the next chunk, io.EOF at completion, or an interruption error.
func (s *PromptStream) Next() (PromptChunk, error) {
	if s.accepted != nil || s.finished {
		return PromptChunk{}, io.EOF
	}
	frame, err := s.stream.Next()
	if err != nil {
		if errors.Is(err, io.EOF) && !s.stream.decoder.terminal {
			return PromptChunk{}, &StreamError{Op: "sendSessionPrompt", Err: io.ErrUnexpectedEOF}
		}
		return PromptChunk{}, err
	}
	var kind struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(frame.Data, &kind); err != nil {
		return PromptChunk{}, &StreamError{Op: "sendSessionPrompt", Err: errors.Join(err, s.Close())}
	}
	if kind.Type == "" {
		return PromptChunk{}, &StreamError{Op: "sendSessionPrompt", Err: errors.Join(errors.New("prompt chunk has no type"), s.Close())}
	}
	if kind.Type == "finish" || kind.Type == "abort" {
		s.finished = true
		if err := s.Close(); err != nil {
			return PromptChunk{}, &StreamError{Op: "sendSessionPrompt", Err: err}
		}
	}
	return PromptChunk{Type: kind.Type, Raw: frame.Data}, nil
}

// Close releases the stream's HTTP response; accepted results own no open body.
func (s *PromptStream) Close() error {
	if s.stream == nil {
		return nil
	}
	return s.stream.Close()
}
