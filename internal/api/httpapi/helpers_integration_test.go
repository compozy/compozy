//go:build integration

package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func decodeHTTPJSON(t *testing.T, resp *http.Response, dest any) {
	t.Helper()

	body, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("read and close HTTP response body: %v", err)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v; body=%s", err, string(body))
	}
}

func closeHTTPBody(t *testing.T, body io.Closer) {
	t.Helper()

	if err := body.Close(); err != nil {
		t.Errorf("close HTTP response body: %v", err)
	}
}

func readAndCloseHTTPBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()

	body, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("read and close HTTP response body: %v", err)
	}
	return body
}

func readHTTPBody(t *testing.T, body io.Reader) []byte {
	t.Helper()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read HTTP response body: %v", err)
	}
	return data
}

const liveSSEJoinTimeout = 2 * time.Second

type liveSSEStreamResult struct {
	err error
}

type liveSSEStream struct {
	cancel  context.CancelFunc
	records <-chan sseRecord
	result  <-chan liveSSEStreamResult
}

func collectLiveSSE(t *testing.T, body io.ReadCloser, want int, timeout time.Duration) []sseRecord {
	t.Helper()

	stream := startLiveSSEStream(t.Context(), body, want+1)
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	records := make([]sseRecord, 0, want)
	var primaryErr error
	var terminalResult *liveSSEStreamResult
	for len(records) < want && primaryErr == nil && terminalResult == nil {
		select {
		case record, ok := <-stream.records:
			if !ok {
				continue
			}
			records = append(records, record)
		case result := <-stream.result:
			terminalResult = &result
			for record := range stream.records {
				records = append(records, record)
			}
			if len(records) < want {
				primaryErr = fmt.Errorf("live SSE ended after %d records; want %d", len(records), want)
			}
		case <-deadline.C:
			primaryErr = fmt.Errorf("timed out waiting for %d SSE records; got %d", want, len(records))
		}
	}

	stream.cancel()
	if terminalResult == nil {
		joinCtx, cancelJoin := context.WithTimeout(context.WithoutCancel(t.Context()), liveSSEJoinTimeout)
		defer cancelJoin()
		select {
		case result := <-stream.result:
			terminalResult = &result
		case <-joinCtx.Done():
			primaryErr = errors.Join(
				primaryErr,
				fmt.Errorf("live SSE reader did not terminate: %w", joinCtx.Err()),
			)
		}
	}
	if terminalResult != nil {
		primaryErr = errors.Join(primaryErr, terminalResult.err)
	}
	if primaryErr != nil {
		t.Fatalf("collect live SSE: %v", primaryErr)
	}
	return records
}

func startLiveSSEStream(ctx context.Context, body io.ReadCloser, buffer int) liveSSEStream {
	streamCtx, cancel := context.WithCancel(ctx)
	recordCh := make(chan sseRecord, buffer)
	resultCh := make(chan liveSSEStreamResult, 1)
	go readLiveSSEStream(streamCtx, body, recordCh, resultCh)
	return liveSSEStream{cancel: cancel, records: recordCh, result: resultCh}
}

func readLiveSSEStream(
	ctx context.Context,
	body io.ReadCloser,
	recordCh chan<- sseRecord,
	resultCh chan<- liveSSEStreamResult,
) {
	defer close(recordCh)

	var closeOnce sync.Once
	var closeErr error
	closeBody := func() {
		closeOnce.Do(func() {
			closeErr = body.Close()
		})
	}
	stopCloseOnCancel := context.AfterFunc(ctx, closeBody)
	emit := func(record sseRecord) bool {
		select {
		case recordCh <- record:
			return true
		case <-ctx.Done():
			return false
		}
	}

	scanner := bufio.NewScanner(body)
	current := sseRecord{}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if !emit(current) {
				break
			}
			current = sseRecord{}
			continue
		}
		switch {
		case strings.HasPrefix(line, "id: "):
			current.ID = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			current.Event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			current.Data = append(current.Data, []byte(strings.TrimPrefix(line, "data: "))...)
		}
	}
	if ctx.Err() == nil && (current.Event != "" || current.ID != "" || len(current.Data) > 0) {
		emit(current)
	}
	scanErr := scanner.Err()
	if ctx.Err() != nil {
		scanErr = nil
	}
	stopCloseOnCancel()
	closeBody()
	resultCh <- liveSSEStreamResult{err: errors.Join(scanErr, closeErr)}
}

func mustHTTPRequest(
	t *testing.T,
	client *http.Client,
	method, url string,
	body []byte,
	headers map[string]string,
) *http.Response {
	t.Helper()

	var reader io.Reader
	if len(body) > 0 {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	return resp
}
