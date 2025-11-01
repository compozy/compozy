package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/compozy/compozy/pkg/logger"

	"github.com/compozy/compozy/sdk/v2/internal/sse"
)

func (c *Client) openStream(
	ctx context.Context,
	path string,
	query url.Values,
	opts *StreamOptions,
	execID string,
	execURL string,
) (*StreamSession, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	resp, err := c.executeStreamRequest(ctx, path, query, opts)
	if err != nil {
		return nil, err
	}
	streamCtx, cancel := context.WithCancel(ctx)
	decoder := sse.NewDecoder(resp.Body)
	events := make(chan StreamEvent)
	errorsCh := make(chan error, 1)
	var once sync.Once
	closeFn := func() error {
		once.Do(cancel)
		return closeBody(resp.Body)
	}
	go c.consumeStream(streamCtx, decoder, events, errorsCh, closeFn)
	return newStreamSession(execID, execURL, events, errorsCh, closeFn), nil
}

func (c *Client) executeStreamRequest(
	ctx context.Context,
	path string,
	query url.Values,
	opts *StreamOptions,
) (*http.Response, error) {
	req, err := c.buildStreamRequest(ctx, path, query, opts)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stream request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		if _, decodeErr := decodeEnvelope[struct{}](resp, http.StatusOK); decodeErr != nil {
			return nil, decodeErr
		}
		return nil, &APIError{Status: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected content-type: %s", ct)
	}
	return resp, nil
}

func (c *Client) buildStreamRequest(
	ctx context.Context,
	path string,
	query url.Values,
	opts *StreamOptions,
) (*http.Request, error) {
	requestURL := c.resolve(path, applyStreamQuery(query, opts))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create stream request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	if opts != nil && opts.LastEventID != nil {
		req.Header.Set("Last-Event-ID", fmt.Sprintf("%d", *opts.LastEventID))
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return req, nil
}

func (c *Client) consumeStream(
	ctx context.Context,
	decoder *sse.Decoder,
	out chan<- StreamEvent,
	errs chan<- error,
	closeFn func() error,
) {
	log := logger.FromContext(ctx)
	defer func() {
		if closeErr := closeFn(); closeErr != nil && log != nil {
			log.Warn("stream close error", "error", closeErr)
		}
		close(out)
		close(errs)
	}()
	for {
		event, err := decoder.Next(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				errs <- err
				return
			}
			if errors.Is(err, io.EOF) {
				errs <- err
				return
			}
			if log != nil {
				log.Warn("stream read error", "error", err)
			}
			errs <- err
			return
		}
		data := make([]byte, len(event.Data))
		copy(data, event.Data)
		select {
		case <-ctx.Done():
			errs <- ctx.Err()
			return
		case out <- StreamEvent{ID: event.ID, Type: event.Type, Data: data}:
		}
	}
}

func applyStreamQuery(values url.Values, opts *StreamOptions) url.Values {
	clone := url.Values{}
	for k, v := range values {
		clone[k] = append([]string(nil), v...)
	}
	if opts == nil {
		return clone
	}
	if opts.PollInterval > 0 {
		millis := opts.PollInterval.Milliseconds()
		if millis > 0 {
			clone.Set("poll_ms", fmt.Sprintf("%d", millis))
		}
	}
	if len(opts.Events) > 0 {
		trimmed := make([]string, 0, len(opts.Events))
		for _, evt := range opts.Events {
			token := strings.TrimSpace(evt)
			if token != "" {
				trimmed = append(trimmed, token)
			}
		}
		if len(trimmed) > 0 {
			clone.Set("events", strings.Join(trimmed, ","))
		}
	}
	return clone
}
