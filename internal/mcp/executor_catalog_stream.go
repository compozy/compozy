package mcp

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const catalogStreamHandshakeTimeout = time.Second

// catalogNotificationStream observes the stream the SDK owns, including silent subscription loss.
type catalogNotificationStream struct {
	next  http.RoundTripper
	ended func()
	body  atomic.Pointer[catalogNotificationBody]
}

func (s *catalogNotificationStream) live() bool {
	body := s.body.Load()
	return body != nil && body.acknowledged.Load()
}

func (s *catalogNotificationStream) observeAcknowledgement(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
	return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		if method == "notifications/subscriptions/acknowledged" {
			if body := s.body.Load(); body != nil {
				body.acknowledged.Store(true)
			}
		}
		return next(ctx, method, req)
	}
}

func (s *catalogNotificationStream) RoundTrip(req *http.Request) (*http.Response, error) {
	if !isCatalogListenRequest(req) {
		return s.next.RoundTrip(req)
	}
	ctx, cancel := context.WithCancel(req.Context())
	timer := time.AfterFunc(catalogStreamHandshakeTimeout, cancel)
	resp, err := s.next.RoundTrip(req.WithContext(ctx))
	timer.Stop()
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
	s.body.Store(body)
	resp.Body = body
	return resp, nil
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
