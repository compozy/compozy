package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultCatalogStreamHandshakeTimeout = time.Second

type catalogRetirement struct{ ctx context.Context }

// catalogNotificationStream observes the stream the SDK owns, including silent subscription loss.
type catalogNotificationStream struct {
	next             http.RoundTripper
	handshakeTimeout time.Duration
	retirement       atomic.Pointer[catalogRetirement]
	revoked          atomic.Bool
	established      func()
	ended            func()
	body             atomic.Pointer[catalogNotificationBody]
}

func (s *catalogNotificationStream) live() bool { return s.liveBody() != nil }

func (s *catalogNotificationStream) liveBody() *catalogNotificationBody {
	body := s.body.Load()
	if !s.revoked.Load() && body != nil && body.acknowledged.Load() {
		return body
	}
	return nil
}

func (s *catalogNotificationStream) observeAcknowledgement(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
	return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		if method == "notifications/subscriptions/acknowledged" && req != nil {
			params, ok := req.GetParams().(*mcpsdk.SubscriptionsAcknowledgedParams)
			if body := s.body.Load(); ok && params != nil && params.Notifications.ToolsListChanged &&
				body != nil && body.acknowledged.CompareAndSwap(false, true) {
				s.signalEstablished()
			}
		}
		return next(ctx, method, req)
	}
}

func (s *catalogNotificationStream) signalEstablished() {
	if s.established != nil {
		s.established()
	}
}

func (s *catalogNotificationStream) RoundTrip(req *http.Request) (*http.Response, error) {
	if retirement := s.retirement.Load(); req.Method == http.MethodDelete && retirement != nil {
		return s.deleteSession(retirement.ctx, req)
	}
	if !isCatalogListenRequest(req) {
		return s.next.RoundTrip(req)
	}
	ctx, cancel := context.WithCancel(req.Context())
	timer := time.AfterFunc(s.handshakeTimeout, cancel)
	resp, err := s.next.RoundTrip(req.WithContext(ctx))
	if !timer.Stop() {
		cancel()
		if resp != nil && resp.Body != nil {
			err = errors.Join(err, resp.Body.Close())
		}
		return nil, errors.Join(context.DeadlineExceeded, err)
	}
	if err != nil {
		cancel()
		return nil, err
	}
	contentType, _, mimeErr := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mimeErr != nil || resp.StatusCode != http.StatusOK || contentType != "text/event-stream" || resp.Body == nil {
		if resp.Body == nil {
			cancel()
		} else {
			resp.Body = &catalogNotificationBody{ReadCloser: resp.Body, end: cancel}
		}
		return resp, nil
	}
	body := &catalogNotificationBody{ReadCloser: resp.Body}
	body.end = func() {
		cancel()
		if s.body.CompareAndSwap(body, nil) {
			s.ended()
		}
	}
	// A legacy GET has no acknowledgement message. Its accepted SSE response opens the channel.
	body.acknowledged.Store(req.Method == http.MethodGet)
	previous := s.body.Swap(body)
	if body.acknowledged.Load() {
		s.signalEstablished()
	} else if previous != nil {
		s.ended()
	}
	resp.Body = body
	return resp, nil
}

func (s *catalogNotificationStream) deleteSession(
	retirement context.Context,
	req *http.Request,
) (*http.Response, error) {
	if err := retirement.Err(); err != nil {
		return nil, err
	}
	// The SDK closes the response without reading its body after receiving the DELETE headers.
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(retirement, cancel)
	defer stop()
	defer cancel()
	return s.next.RoundTrip(req.WithContext(ctx))
}

func isCatalogListenRequest(req *http.Request) bool {
	if req.Method == http.MethodGet {
		return true
	}
	if req.Method != http.MethodPost || req.GetBody == nil {
		return false
	}
	body, err := req.GetBody()
	if err != nil {
		return false
	}
	var rpc struct {
		Method string `json:"method"`
	}
	decodeErr := json.NewDecoder(body).Decode(&rpc)
	return body.Close() == nil && decodeErr == nil && rpc.Method == "subscriptions/listen"
}

type catalogNotificationBody struct {
	io.ReadCloser
	acknowledged atomic.Bool
	once         sync.Once
	end          func()
}

func (b *catalogNotificationBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.once.Do(b.end)
	}
	return n, err
}

func (b *catalogNotificationBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.end)
	return err
}
