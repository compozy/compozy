//go:build integration

package daytona

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	daytonaAPIKeyEnv       = "DAYTONA_API_KEY"
	daytonaAPIURLEnv       = "DAYTONA_API_URL"
	daytonaOrganizationEnv = "DAYTONA_ORGANIZATION_ID"
	daytonaSSHHostEnv      = "DAYTONA_SSH_HOST"
	daytonaValidateSSHEnv  = "DAYTONA_VALIDATE_SSH_GATEWAY"
	defaultDaytonaAPIURL   = "https://app.daytona.io/api"
	defaultDaytonaSSHHost  = "ssh.app.daytona.io"
	sshAccessExpiryMinutes = "60"
	httpClientTimeout      = 2 * time.Minute
	testTimeout            = 5 * time.Minute
	cleanupTimeout         = time.Minute
	sshCommandTimeout      = 45 * time.Second
	sshReadyTimeout        = 30 * time.Second
	sshReadyRetryInterval  = 500 * time.Millisecond
	sshCloseTimeout        = 3 * time.Second
	latencyThreshold       = 100 * time.Millisecond
	maxResponseBodyBytes   = 1 << 20
)

var sshReadyMarker = []byte("__agh_daytona_ssh_ready__")

func TestDaytonaSSHNonPTYValidation(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv(daytonaAPIKeyEnv))
	if apiKey == "" {
		t.Skipf("%s is required for Daytona SSH validation", daytonaAPIKeyEnv)
	}
	if strings.TrimSpace(os.Getenv(daytonaValidateSSHEnv)) == "" {
		t.Skipf(
			"%s is diagnostic-only now that the launcher uses the sidecar transport",
			daytonaValidateSSHEnv,
		)
	}
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skipf("OpenSSH client is required for Daytona SSH validation: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)

	client := newDaytonaValidationClient(apiKey)
	sandboxID, err := client.createSandbox(ctx)
	if err != nil {
		t.Fatalf("create Daytona sandbox: %v", err)
	}
	t.Logf("created Daytona sandbox %q for non-PTY SSH validation", sandboxID)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cleanupCancel()
		if cleanupErr := client.deleteSandbox(cleanupCtx, sandboxID); cleanupErr != nil {
			t.Errorf("delete Daytona sandbox %q: %v", sandboxID, cleanupErr)
		}
	})

	sshAccess, err := client.createSSHAccess(ctx, sandboxID)
	if err != nil {
		t.Fatalf("create SSH access for Daytona sandbox %q: %v", sandboxID, err)
	}
	target := sshAccess.Token + "@" + daytonaSSHHost()

	session, attempts, err := openSSHCatSession(ctx, target)
	if err != nil {
		t.Fatalf("open Daytona SSH validation session: %v", err)
	}
	if attempts > 1 {
		t.Logf("Daytona SSH gateway became ready after %d attempts", attempts)
	}
	t.Cleanup(func() {
		trailing, timedOut, cleanupErr := session.Close()
		if timedOut {
			t.Logf(
				"Daytona SSH validation session did not exit after stdin close within %s; terminated local client",
				sshCloseTimeout,
			)
		}
		if cleanupErr != nil {
			t.Errorf("close Daytona SSH validation session: %v", cleanupErr)
		}
		if len(trailing) != 0 {
			t.Errorf("SSH stdout included trailing bytes after session close: %s", previewBytes(trailing))
		}
		if containsTerminalArtifact(trailing) {
			t.Errorf("SSH stdout trailing bytes contain terminal artifact bytes: %s", previewBytes(trailing))
		}
	})

	runPayloadChecks(t, session)
	runLatencyCheck(t, session)
}

type daytonaValidationClient struct {
	apiKey         string
	apiURL         string
	organizationID string
	httpClient     *http.Client
}

type sshAccessResponse struct {
	Token string `json:"token"`
}

type sshRoundTripResult struct {
	output  []byte
	latency time.Duration
}

type sshCatSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr *lockedBuffer
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newDaytonaValidationClient(apiKey string) daytonaValidationClient {
	apiURL := strings.TrimRight(strings.TrimSpace(os.Getenv(daytonaAPIURLEnv)), "/")
	if apiURL == "" {
		apiURL = defaultDaytonaAPIURL
	}
	return daytonaValidationClient{
		apiKey:         apiKey,
		apiURL:         apiURL,
		organizationID: strings.TrimSpace(os.Getenv(daytonaOrganizationEnv)),
		httpClient:     &http.Client{Timeout: httpClientTimeout},
	}
}

func (c daytonaValidationClient) createSandbox(ctx context.Context) (string, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodPost, []string{"sandbox"}, nil, map[string]string{}, &raw); err != nil {
		return "", err
	}
	sandboxID, err := extractSandboxID(raw)
	if err != nil {
		return "", fmt.Errorf("extract sandbox id from create response: %w", err)
	}
	return sandboxID, nil
}

func (c daytonaValidationClient) createSSHAccess(ctx context.Context, sandboxID string) (sshAccessResponse, error) {
	query := url.Values{"expiresInMinutes": []string{sshAccessExpiryMinutes}}
	var response sshAccessResponse
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		[]string{"sandbox", sandboxID, "ssh-access"},
		query,
		nil,
		&response,
	); err != nil {
		return sshAccessResponse{}, err
	}
	if strings.TrimSpace(response.Token) == "" {
		return sshAccessResponse{}, errors.New("Daytona ssh-access response did not include token")
	}
	return response, nil
}

func (c daytonaValidationClient) deleteSandbox(ctx context.Context, sandboxID string) error {
	return c.doJSON(ctx, http.MethodDelete, []string{"sandbox", sandboxID}, nil, nil, nil)
}

func (c daytonaValidationClient) doJSON(
	ctx context.Context,
	method string,
	pathParts []string,
	query url.Values,
	body any,
	out any,
) (err error) {
	endpoint, err := c.endpoint(pathParts, query)
	if err != nil {
		return err
	}
	var bodyReader io.Reader
	if body != nil {
		encoded, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return fmt.Errorf("marshal Daytona request body: %w", marshalErr)
		}
		bodyReader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return fmt.Errorf("create Daytona request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if c.organizationID != "" {
		req.Header.Set("X-Daytona-Organization-ID", c.organizationID)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send Daytona %s request: %w", method, err)
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil && err == nil {
			err = fmt.Errorf("close Daytona response body: %w", closeErr)
		}
	}()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return fmt.Errorf("read Daytona response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf(
			"Daytona %s %s returned status %d: %s",
			method,
			req.URL.Redacted(),
			resp.StatusCode,
			strings.TrimSpace(string(responseBody)),
		)
	}
	if out == nil || len(bytes.TrimSpace(responseBody)) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, out); err != nil {
		return fmt.Errorf("decode Daytona response body: %w", err)
	}
	return nil
}

func (c daytonaValidationClient) endpoint(pathParts []string, query url.Values) (string, error) {
	base, err := url.Parse(c.apiURL + "/")
	if err != nil {
		return "", fmt.Errorf("parse Daytona API URL: %w", err)
	}
	endpoint := base.JoinPath(pathParts...)
	if query != nil {
		endpoint.RawQuery = query.Encode()
	}
	return endpoint.String(), nil
}

func runPayloadChecks(t *testing.T, session *sshCatSession) {
	t.Helper()

	cases := []struct {
		name    string
		payload []byte
	}{
		{name: "small-100B", payload: mustJSONPayload(t, 100)},
		{name: "medium-10KB", payload: mustJSONPayload(t, 10*1024)},
		{name: "large-100KB", payload: mustJSONPayload(t, 100*1024)},
		{name: "newline-delimited-json", payload: newlineDelimitedJSONPayload(t)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := session.roundTrip(tc.payload)
			if err != nil {
				t.Fatalf("SSH non-PTY cat round trip failed: %v", err)
			}
			assertCleanRoundTrip(t, tc.payload, result)
			t.Logf("payload=%s bytes=%d latency=%s artifacts=none", tc.name, len(tc.payload), result.latency)
		})
	}
}

func runLatencyCheck(t *testing.T, session *sshCatSession) {
	t.Helper()

	payload := mustJSONPayload(t, 1024)
	result, err := session.roundTrip(payload)
	if err != nil {
		t.Fatalf("SSH non-PTY latency check failed: %v", err)
	}
	assertCleanRoundTrip(t, payload, result)
	if result.latency > latencyThreshold {
		t.Fatalf("1KB round-trip latency = %s, want <= %s", result.latency, latencyThreshold)
	}
	t.Logf("payload=latency-1KB bytes=%d latency=%s threshold=%s", len(payload), result.latency, latencyThreshold)
}

func openSSHCatSession(ctx context.Context, target string) (*sshCatSession, int, error) {
	deadline := time.Now().Add(sshReadyTimeout)
	attempts := 0
	var lastErr error
	for {
		attempts++
		session, err := openSSHCatSessionAttempt(ctx, target)
		if err == nil {
			return session, attempts, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, attempts, fmt.Errorf("wait for Daytona SSH readiness: %w", ctx.Err())
		}
		if time.Now().After(deadline) {
			return nil, attempts, fmt.Errorf("wait for Daytona SSH readiness: %w", lastErr)
		}
		timer := time.NewTimer(sshReadyRetryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, attempts, fmt.Errorf("wait for Daytona SSH readiness: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func openSSHCatSessionAttempt(ctx context.Context, target string) (*sshCatSession, error) {
	cmd := exec.CommandContext(ctx, "ssh", sshCommandArgs(target)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open ssh stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open ssh stdout pipe: %w", err)
	}
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ssh non-PTY cat session: %w", err)
	}
	if err := readReadyMarker(stdout); err != nil {
		closeErr := stdin.Close()
		return nil, waitWithSSHError(cmd, errors.Join(err, closeErr), stderr.String())
	}
	return &sshCatSession{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr}, nil
}

func (s *sshCatSession) roundTrip(payload []byte) (sshRoundTripResult, error) {
	started := time.Now()
	writeErrCh := make(chan error, 1)
	go func() {
		writeErrCh <- writeAll(s.stdin, payload)
	}()

	output := make([]byte, len(payload))
	_, readErr := io.ReadFull(s.stdout, output)
	latency := time.Since(started)
	writeErr := <-writeErrCh

	if err := errors.Join(writeErr, readErr); err != nil {
		return sshRoundTripResult{}, sshError(err, s.stderr.String())
	}
	return sshRoundTripResult{output: output, latency: latency}, nil
}

func (s *sshCatSession) Close() ([]byte, bool, error) {
	if s == nil {
		return nil, false, nil
	}
	closeErr := s.stdin.Close()
	type closeResult struct {
		trailing []byte
		err      error
	}
	done := make(chan closeResult, 1)
	go func() {
		trailing, readErr := io.ReadAll(s.stdout)
		waitErr := s.cmd.Wait()
		if err := errors.Join(closeErr, readErr, waitErr); err != nil {
			done <- closeResult{trailing: trailing, err: sshError(err, s.stderr.String())}
			return
		}
		done <- closeResult{trailing: trailing}
	}()

	timer := time.NewTimer(sshCloseTimeout)
	defer timer.Stop()

	select {
	case result := <-done:
		return result.trailing, false, result.err
	case <-timer.C:
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		result := <-done
		return result.trailing, true, nil
	}
}

func sshCommandArgs(target string) []string {
	return []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "RequestTTY=no",
		"-T",
		target,
		fmt.Sprintf("sh -c 'printf %s; exec cat'", string(sshReadyMarker)),
	}
}

func readReadyMarker(stdout io.Reader) error {
	marker := make([]byte, len(sshReadyMarker))
	if _, err := io.ReadFull(stdout, marker); err != nil {
		return fmt.Errorf("read ssh ready marker: %w", err)
	}
	if !bytes.Equal(marker, sshReadyMarker) {
		return fmt.Errorf("unexpected ssh ready marker %q", string(marker))
	}
	return nil
}

func waitWithSSHError(cmd *exec.Cmd, cause error, stderr string) error {
	waitErr := cmd.Wait()
	return sshError(errors.Join(cause, waitErr), stderr)
}

func sshError(err error, stderr string) error {
	if strings.TrimSpace(stderr) == "" {
		return err
	}
	return fmt.Errorf("%w; ssh stderr: %s", err, strings.TrimSpace(stderr))
}

func writeAll(writer io.Writer, payload []byte) error {
	for len(payload) > 0 {
		n, err := writer.Write(payload)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		payload = payload[n:]
	}
	return nil
}

func assertCleanRoundTrip(t *testing.T, want []byte, result sshRoundTripResult) {
	t.Helper()

	if containsTerminalArtifact(result.output) {
		t.Fatalf("SSH stdout contains terminal artifact bytes: %s", previewBytes(result.output))
	}
	if !bytes.Equal(result.output, want) {
		t.Fatalf("SSH stdout mismatch\ngot:  %s\nwant: %s", previewBytes(result.output), previewBytes(want))
	}
}

func containsTerminalArtifact(output []byte) bool {
	return bytes.ContainsAny(output, "\x1b\r\b\x7f")
}

func mustJSONPayload(t *testing.T, size int) []byte {
	t.Helper()

	payload, err := jsonPayload(size)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func jsonPayload(size int) ([]byte, error) {
	const prefix = `{"jsonrpc":"2.0","id":1,"method":"validate","params":{"padding":"`
	const suffix = `"}}`

	paddingSize := size - len(prefix) - len(suffix)
	if paddingSize < 0 {
		return nil, fmt.Errorf("JSON payload size %d is smaller than envelope size %d", size, len(prefix)+len(suffix))
	}
	payload := prefix + strings.Repeat("x", paddingSize) + suffix
	return []byte(payload), nil
}

func newlineDelimitedJSONPayload(t *testing.T) []byte {
	t.Helper()

	messages := [][]byte{
		mustJSONPayload(t, 128),
		mustJSONPayload(t, 512),
		mustJSONPayload(t, 1024),
	}
	return append(bytes.Join(messages, []byte("\n")), '\n')
}

func previewBytes(data []byte) string {
	const maxPreviewBytes = 256
	if len(data) <= maxPreviewBytes {
		return fmt.Sprintf("%q", data)
	}
	return fmt.Sprintf("%q... (%d bytes)", data[:maxPreviewBytes], len(data))
}

func extractSandboxID(raw json.RawMessage) (string, error) {
	var response map[string]any
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", fmt.Errorf("decode create sandbox response: %w", err)
	}
	if sandboxID := stringField(response, "id", "sandboxId", "sandbox_id", "name"); sandboxID != "" {
		return sandboxID, nil
	}
	for _, nestedKey := range []string{"sandbox", "data", "result"} {
		nested, ok := response[nestedKey].(map[string]any)
		if !ok {
			continue
		}
		if sandboxID := stringField(nested, "id", "sandboxId", "sandbox_id", "name"); sandboxID != "" {
			return sandboxID, nil
		}
	}
	return "", fmt.Errorf("missing sandbox identifier in response keys: %v", mapKeys(response))
}

func stringField(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key].(string)
		if ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func daytonaSSHHost() string {
	host := strings.TrimSpace(os.Getenv(daytonaSSHHostEnv))
	if host == "" {
		return defaultDaytonaSSHHost
	}
	return host
}
