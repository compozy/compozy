package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/testutil"
)

// authTestEnv builds a helper-peer environment without duplicating HOME or the
// helper keys, so the record file observes exactly the configured home.
func authTestEnv(
	t *testing.T,
	scenario string,
	recordPath string,
	capturePath string,
	home string,
) []string {
	t.Helper()

	drop := map[string]bool{
		"HOME": true, "USERPROFILE": true,
		testHelperEnvKey: true, testHelperScenarioKey: true,
		testHelperFileKey: true, testHelperCaptureKey: true,
		testWrapperEnvKey: true, "COMPOZY_TEST_AUTH_MARKER": true,
	}
	env := make([]string, 0, len(os.Environ())+8)
	for _, entry := range os.Environ() {
		if key, _, ok := strings.Cut(entry, "="); ok && drop[key] {
			continue
		}
		env = append(env, entry)
	}
	env = append(env,
		testHelperEnvKey+"=1",
		testHelperScenarioKey+"="+scenario,
		testHelperFileKey+"="+recordPath,
		"HOME="+home,
		"COMPOZY_TEST_AUTH_MARKER=marker-value",
	)
	if strings.TrimSpace(capturePath) != "" {
		env = append(env, testHelperCaptureKey+"="+capturePath)
	}
	return env
}

func authTestRequest(
	t *testing.T,
	scenario string,
	home string,
	recordPath string,
	capturePath string,
) (AuthenticationRequest, string) {
	t.Helper()

	if strings.TrimSpace(home) == "" {
		home = t.TempDir()
	}
	cwd := t.TempDir()
	return AuthenticationRequest{
		SessionInspectionRequest: SessionInspectionRequest{
			AgentName: "auth-probe",
			Command:   helperCommand(t),
			Cwd:       cwd,
			Env:       authTestEnv(t, scenario, recordPath, capturePath, home),
		},
		MethodID: "oauth-personal",
	}, cwd
}

func readAuthRecord(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	lines := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func authRecordHas(lines []string, prefix string) (string, bool) {
	for _, line := range lines {
		if value, ok := strings.CutPrefix(line, prefix); ok {
			return value, true
		}
	}
	return "", false
}

// captureAuthSequence returns the client-to-peer method names in send order.
func captureAuthSequence(t *testing.T, capturePath string) []string {
	t.Helper()

	data, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", capturePath, err)
	}
	sequence := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var envelope capturedRequestEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(captured envelope) error = %v", err)
		}
		sequence = append(sequence, envelope.Method)
	}
	return sequence
}

func assertAuthSequence(t *testing.T, got, want []string) {
	t.Helper()

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("request sequence = %v, want %v", got, want)
	}
}

// authPeerGone asserts the helper process exited instead of lingering as an
// orphan. The exit record line proves clean stdin-close shutdown; the PID
// check also covers peers killed out of a blocked call.
func authPeerGone(t *testing.T, recordPath string) {
	t.Helper()

	lines := readAuthRecord(t, recordPath)
	pidValue, ok := authRecordHas(lines, "pid ")
	if !ok {
		t.Fatal("record file does not contain helper pid")
	}
	var pid int
	if _, err := fmt.Sscanf(pidValue, "%d", &pid); err != nil || pid <= 0 {
		t.Fatalf("parse helper pid %q: %v", pidValue, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("helper process %d still running", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

type syncWriter struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *syncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func TestAuthenticateCleanLoginOrdering(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "record.log")
	capturePath := filepath.Join(dir, "capture.jsonl")
	home := filepath.Join(dir, "operator-home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	output := &syncWriter{}
	req, _ := authTestRequest(t, "auth_raw_login_clean", home, recordPath, capturePath)
	req.InteractiveOutput = output

	if err := Authenticate(ctx, req); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	assertAuthSequence(t, captureAuthSequence(t, capturePath), []string{
		"initialize", "session/new", "authenticate", "session/new", "session/close",
	})
	authenticate := captureRequestParams(t, capturePath, "authenticate")
	if got := string(authenticate["methodId"]); got != `"oauth-personal"` {
		t.Fatalf("authenticate methodId = %s, want %q", got, "oauth-personal")
	}
	// The readiness probe is prompt/tool-free: session/new carries an empty MCP
	// list (not null) and no hosted tools, and no prompt, load, mode, config,
	// filesystem, terminal, or permission traffic may appear.
	for _, params := range captureRequestParamsForMethod(t, capturePath, "session/new") {
		for key := range params {
			switch key {
			case "cwd", "mcpServers", "additional_dirs", "_meta":
			default:
				t.Fatalf("session/new param %q carries unexpected surface", key)
			}
		}
		if raw, _ := json.Marshal(params["mcpServers"]); string(raw) != "[]" {
			t.Fatalf("session/new mcpServers = %s, want empty list", raw)
		}
	}
	lines := readAuthRecord(t, recordPath)
	if _, ok := authRecordHas(lines, "exit"); !ok {
		t.Fatal("helper did not exit after Authenticate")
	}
	if got := output.String(); !strings.Contains(got, "auth peer starting login_clean") {
		t.Fatalf("interactive output = %q, want live peer progress", got)
	}
	authPeerGone(t, recordPath)
}

func TestAuthenticateRepeatedLoginSkipsConsent(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "record.log")
	capturePath := filepath.Join(dir, "capture.jsonl")
	req, _ := authTestRequest(t, "auth_raw_login_ready", "", recordPath, capturePath)

	if err := Authenticate(ctx, req); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	assertAuthSequence(t, captureAuthSequence(t, capturePath), []string{
		"initialize", "session/new", "session/close",
	})
}

func TestAuthenticateWithoutAdvertisedClose(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "record.log")
	capturePath := filepath.Join(dir, "capture.jsonl")
	req, _ := authTestRequest(t, "auth_raw_login_no_close", "", recordPath, capturePath)

	if err := Authenticate(ctx, req); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	assertAuthSequence(t, captureAuthSequence(t, capturePath), []string{
		"initialize", "session/new", "authenticate", "session/new",
	})
	authPeerGone(t, recordPath)
}

func TestAuthenticateMethodSafety(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		scenario string
		methodID string
		reason   AuthMethodUnavailableReason
		wantAuth bool
	}{
		{
			name:     "Should authenticate the omitted-type agent method",
			scenario: "auth_raw_login_clean",
			methodID: "oauth-personal",
			wantAuth: true,
		},
		{
			name:     "Should authenticate the explicit agent type",
			scenario: "auth_raw_method_agent_type",
			methodID: "oauth-personal",
			wantAuth: true,
		},
		{
			name:     "Should refuse the absent method without authenticating",
			scenario: "auth_raw_method_absent",
			methodID: "oauth-personal",
			reason:   AuthMethodUnavailableAbsent,
		},
		{
			name:     "Should refuse the terminal method without authenticating",
			scenario: "auth_raw_method_terminal",
			methodID: "oauth-personal",
			reason:   AuthMethodUnavailableTerminal,
		},
		{
			name:     "Should refuse the unknown method without authenticating",
			scenario: "auth_raw_method_unknown",
			methodID: "oauth-personal",
			reason:   AuthMethodUnavailableUnknown,
		},
		{
			name:     "Should refuse the known non-agent method without authenticating",
			scenario: "auth_raw_method_env_var",
			methodID: "oauth-personal",
			reason:   AuthMethodUnavailableUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			dir := t.TempDir()
			recordPath := filepath.Join(dir, "record.log")
			capturePath := filepath.Join(dir, "capture.jsonl")
			req, _ := authTestRequest(t, tt.scenario, "", recordPath, capturePath)
			req.MethodID = tt.methodID

			err := Authenticate(ctx, req)
			sequence := captureAuthSequence(t, capturePath)
			for _, method := range sequence {
				if method == "authenticate" && !tt.wantAuth {
					t.Fatalf("authenticate was sent for %s %q", tt.scenario, tt.methodID)
				}
			}
			if tt.wantAuth {
				if err != nil {
					t.Fatalf("Authenticate() error = %v", err)
				}
				if !strings.Contains(strings.Join(sequence, ","), "authenticate") {
					t.Fatalf("authenticate missing from %v", sequence)
				}
				return
			}
			var unavailable *AuthMethodUnavailableError
			if !errors.As(err, &unavailable) {
				t.Fatalf("Authenticate() error = %v, want AuthMethodUnavailableError", err)
			}
			if unavailable.Reason != tt.reason {
				t.Fatalf("unavailable reason = %q, want %q", unavailable.Reason, tt.reason)
			}
			if strings.Contains(err.Error(), "tok-fake-secret-0") {
				t.Fatalf("method error leaks peer output: %v", err)
			}
			authPeerGone(t, recordPath)
		})
	}
}

func TestAuthenticateRejectsUnshapedMethod(t *testing.T) {
	t.Parallel()

	for _, methodID := range []string{"", "   ", "oauth-personal\n", "oauth\x00personal"} {
		req := AuthenticationRequest{MethodID: methodID}
		if err := Authenticate(testutil.Context(t), req); err == nil {
			t.Fatalf("Authenticate(%q) error = nil, want validation failure", methodID)
		}
	}
}

func TestProbeAuthentication(t *testing.T) {
	t.Parallel()

	t.Run("Should report ready without authenticating", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		dir := t.TempDir()
		recordPath := filepath.Join(dir, "record.log")
		capturePath := filepath.Join(dir, "capture.jsonl")
		req, _ := authTestRequest(t, "auth_raw_login_ready", "", recordPath, capturePath)

		if err := ProbeAuthentication(ctx, req.SessionInspectionRequest); err != nil {
			t.Fatalf("ProbeAuthentication() error = %v", err)
		}
		assertAuthSequence(t, captureAuthSequence(t, capturePath), []string{
			"initialize", "session/new", "session/close",
		})
		for _, params := range captureRequestParamsForMethod(t, capturePath, "session/new") {
			if raw, _ := json.Marshal(params["mcpServers"]); string(raw) != "[]" {
				t.Fatalf("session/new mcpServers = %s, want empty list", raw)
			}
		}
		authPeerGone(t, recordPath)
	})

	t.Run("Should surface auth-required without authenticating", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		dir := t.TempDir()
		recordPath := filepath.Join(dir, "record.log")
		capturePath := filepath.Join(dir, "capture.jsonl")
		req, _ := authTestRequest(t, "auth_raw_login_clean", "", recordPath, capturePath)

		err := ProbeAuthentication(ctx, req.SessionInspectionRequest)
		if !IsAuthRequiredError(err) {
			t.Fatalf("ProbeAuthentication() error = %v, want auth-required", err)
		}
		assertAuthSequence(t, captureAuthSequence(t, capturePath), []string{
			"initialize", "session/new",
		})
		authPeerGone(t, recordPath)
	})
}

func TestAuthenticateInterruptedLoginCleansUp(t *testing.T) {
	t.Parallel()

	t.Run("Should join cleanup after cancellation", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		recordPath := filepath.Join(dir, "record.log")
		capturePath := filepath.Join(dir, "capture.jsonl")
		req, _ := authTestRequest(t, "auth_raw_login_block", "", recordPath, capturePath)
		ctx, cancel := context.WithCancel(testutil.Context(t))
		done := make(chan error, 1)
		go func() { done <- Authenticate(ctx, req) }()
		time.Sleep(500 * time.Millisecond)
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Authenticate() error = %v, want cancellation", err)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("Authenticate() did not settle after cancellation")
		}
		authPeerGone(t, recordPath)
	})

	t.Run("Should join cleanup after caller timeout", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		recordPath := filepath.Join(dir, "record.log")
		capturePath := filepath.Join(dir, "capture.jsonl")
		req, _ := authTestRequest(t, "auth_raw_login_block", "", recordPath, capturePath)
		ctx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
		defer cancel()
		start := time.Now()
		err := Authenticate(ctx, req)
		if err == nil {
			t.Fatal("Authenticate() error = nil, want timeout")
		}
		if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			t.Fatalf("Authenticate() error = %v, want deadline", err)
		}
		if elapsed := time.Since(start); elapsed > 25*time.Second {
			t.Fatalf("Authenticate() took %v, want cleanup within the stop budget", elapsed)
		}
		authPeerGone(t, recordPath)
	})
}

func TestAuthenticateRefusesInboundWork(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "record.log")
	capturePath := filepath.Join(dir, "capture.jsonl")
	isolatedHome := filepath.Join(dir, "isolated-home")
	if err := os.MkdirAll(isolatedHome, 0o755); err != nil {
		t.Fatal(err)
	}
	req, _ := authTestRequest(t, "auth_raw_login_refuse", isolatedHome, recordPath, capturePath)

	if err := Authenticate(ctx, req); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	lines := readAuthRecord(t, recordPath)
	if home, ok := authRecordHas(lines, "home "); !ok || home != "HOME="+isolatedHome {
		t.Fatalf("helper home = %q, want isolated %q", home, isolatedHome)
	}
	if marker, ok := authRecordHas(lines, "env COMPOZY_TEST_AUTH_MARKER="); !ok ||
		marker != "marker-value" {
		t.Fatalf("helper marker = %q, want exact launch environment", marker)
	}
	refusals := map[string]string{}
	for _, line := range lines {
		if method, result, ok := strings.Cut(strings.TrimPrefix(line, "inbound "), " "); ok &&
			strings.HasPrefix(line, "inbound ") {
			refusals[method] = result
		}
	}
	for _, method := range []string{"fs/read_text_file", "fs/write_text_file", "terminal/create"} {
		result, ok := refusals[method]
		if !ok {
			t.Fatalf("no inbound record for %s", method)
		}
		if !strings.HasPrefix(result, "error=") {
			t.Fatalf("inbound %s = %q, want error refusal", method, result)
		}
	}
	permission, ok := refusals["session/request_permission"]
	if !ok {
		t.Fatal("no inbound record for session/request_permission")
	}
	if !strings.HasPrefix(permission, "result=") || strings.Contains(permission, "allow") {
		t.Fatalf("permission refusal = %q, want reject without allow", permission)
	}
	artifact, ok := authRecordHas(lines, "artifact ")
	if !ok {
		t.Fatal("refusal probe did not declare its artifact path")
	}
	if _, err := os.Stat(strings.TrimSpace(artifact)); !os.IsNotExist(err) {
		t.Fatalf("refused write created artifact %q", artifact)
	}
	authPeerGone(t, recordPath)
}

func TestIsAuthRequiredError(t *testing.T) {
	t.Parallel()

	if IsAuthRequiredError(nil) {
		t.Fatal("IsAuthRequiredError(nil) = true, want false")
	}
	if IsAuthRequiredError(errors.New("boom")) {
		t.Fatal("IsAuthRequiredError(boom) = true, want false")
	}
	required := fmt.Errorf(
		"acp: create authentication session: %w",
		&acpsdk.RequestError{Code: -32000, Message: "authentication required"},
	)
	if !IsAuthRequiredError(required) {
		t.Fatal("IsAuthRequiredError(wrapped -32000) = false, want true")
	}
	other := fmt.Errorf(
		"acp: create authentication session: %w",
		&acpsdk.RequestError{Code: -32603, Message: "boom"},
	)
	if IsAuthRequiredError(other) {
		t.Fatal("IsAuthRequiredError(wrapped -32603) = true, want false")
	}
}
