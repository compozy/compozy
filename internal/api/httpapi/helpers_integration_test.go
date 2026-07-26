//go:build integration

package httpapi

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func decodeHTTPJSON(t *testing.T, resp *http.Response, dest any) {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("io.ReadAll(response) error = %v", err)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v; body=%s", err, string(body))
	}
}

func collectLiveSSE(t *testing.T, body io.ReadCloser, want int, timeout time.Duration) []sseRecord {
	t.Helper()

	records := make([]sseRecord, 0, want)
	recordCh := make(chan sseRecord, want+1)
	errCh := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(body)
		current := sseRecord{}
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				recordCh <- current
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
		if current.Event != "" || current.ID != "" || len(current.Data) > 0 {
			recordCh <- current
		}
		if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
			errCh <- err
			return
		}
		close(recordCh)
	}()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for len(records) < want {
		select {
		case record, ok := <-recordCh:
			if !ok {
				return records
			}
			records = append(records, record)
		case err := <-errCh:
			t.Fatalf("scan live SSE error = %v", err)
		case <-deadline.C:
			return records
		}
	}

	return records
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
