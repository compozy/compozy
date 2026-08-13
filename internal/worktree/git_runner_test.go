package worktree

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
)

type scriptedGitRunner struct {
	mu     sync.Mutex
	calls  int
	stdout []byte
	stderr []byte
	err    error
}

func (r *scriptedGitRunner) Run(context.Context, string, ...string) ([]byte, []byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return append([]byte(nil), r.stdout...), append([]byte(nil), r.stderr...), r.err
}

func (r *scriptedGitRunner) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// Canonical suite: Git capability and subprocess trust contract.
func TestGitCapabilityAndRunner(t *testing.T) {
	t.Parallel()

	t.Run("Should cap captured output while reporting complete writes", func(t *testing.T) {
		t.Parallel()
		buffer := newCappedBuffer(5)
		for _, value := range [][]byte{[]byte("abc"), []byte("defgh")} {
			written, err := buffer.Write(value)
			if err != nil || written != len(value) {
				t.Fatalf("Write(%q) = %d, %v, want full input length", value, written, err)
			}
		}
		if got := buffer.String(); got != "abcde" {
			t.Fatalf("captured output = %q, want capped abcde", got)
		}
	})

	t.Run("Should coalesce split writes until a complete output line", func(t *testing.T) {
		t.Parallel()
		var outputs []GitOutput
		writer := &gitOutputWriter{
			stream: GitOutputStderr,
			onOutput: func(output GitOutput) {
				outputs = append(outputs, output)
			},
		}
		for _, value := range [][]byte{
			[]byte("claim_token=compozy_"),
			[]byte("claim_hook_secret\nnext"),
		} {
			if _, err := writer.Write(value); err != nil {
				t.Fatalf("Write(%q) error = %v", value, err)
			}
		}
		writer.Flush()
		if len(outputs) != 2 || string(outputs[0].Chunk) != "claim_token=compozy_claim_hook_secret\n" ||
			string(outputs[1].Chunk) != "next" {
			t.Fatalf("output chunks = %#v, want complete line then EOF remainder", outputs)
		}
	})

	t.Run("Should bound captured and unterminated streaming output", func(t *testing.T) {
		t.Parallel()
		var outputs []GitOutput
		writer := &gitOutputWriter{onOutput: func(output GitOutput) { outputs = append(outputs, output) }}
		input := []byte(strings.Repeat("x", maxGitOutputBytes+maxGitStreamChunkBytes))
		written, err := writer.Write(input)
		if err != nil || written != len(input) {
			t.Fatalf("Write(large) = %d, %v; want %d bytes", written, err, len(input))
		}
		capturedWithinLimit := len(writer.Bytes()) == maxGitOutputBytes
		pendingWithinLimit := len(writer.pending) < maxGitStreamChunkBytes
		if !capturedWithinLimit || !pendingWithinLimit || len(outputs) == 0 {
			t.Fatalf(
				"bounded writer captured=%d pending=%d outputs=%d",
				len(writer.Bytes()),
				len(writer.pending),
				len(outputs),
			)
		}
	})

	t.Run("Should report a missing Git executable", func(t *testing.T) {
		t.Parallel()
		gate := NewCapabilityGateWithLookPath(&scriptedGitRunner{}, func(string) (string, error) {
			return "", os.ErrNotExist
		})
		diagnostic, err := gate.Check(context.Background())
		if !errors.Is(err, ErrGitUnavailable) || diagnostic.Code != ErrGitUnavailable.Error() {
			t.Fatalf("Check() = (%#v, %v), want worktree_git_unavailable", diagnostic, err)
		}
	})

	t.Run("Should reject unavailable capability dependencies and failed probes", func(t *testing.T) {
		t.Parallel()
		var nilGate *CapabilityGate
		if diagnostic, err := nilGate.Check(
			context.Background(),
		); !errors.Is(err, ErrGitUnavailable) ||
			diagnostic.Code == "" {
			t.Fatalf("nil Check() = (%#v, %v), want unavailable", diagnostic, err)
		}
		cases := []struct {
			name string
			gate *CapabilityGate
		}{
			{name: "look path", gate: &CapabilityGate{runner: &scriptedGitRunner{}}},
			{name: "runner", gate: NewCapabilityGateWithLookPath(nil, func(string) (string, error) {
				return "/usr/bin/git", nil
			})},
			{name: "probe", gate: NewCapabilityGateWithLookPath(
				&scriptedGitRunner{stderr: []byte("probe failed"), err: errors.New("exit 1")},
				func(string) (string, error) { return "/usr/bin/git", nil },
			)},
		}
		for _, testCase := range cases {
			t.Run("Should reject missing "+testCase.name, func(t *testing.T) {
				t.Parallel()
				diagnostic, err := testCase.gate.Check(context.Background())
				if !errors.Is(err, ErrGitUnavailable) || diagnostic.Code != ErrGitUnavailable.Error() {
					t.Fatalf("Check() = (%#v, %v), want unavailable", diagnostic, err)
				}
			})
		}
	})

	t.Run("Should redact capability probe diagnostics", func(t *testing.T) {
		t.Parallel()
		secret := "ghp_abcdefghijklmnopqrstuvwxyz0123456789"
		gate := NewCapabilityGateWithLookPath(&scriptedGitRunner{}, func(string) (string, error) {
			return "", errors.New("lookup failed with " + secret)
		})
		diagnostic, err := gate.Check(context.Background())
		if !errors.Is(err, ErrGitUnavailable) || strings.Contains(diagnostic.Message, secret) ||
			strings.Contains(err.Error(), secret) {
			t.Fatalf("Check(secret) = (%#v, %v), want redacted unavailable diagnostic", diagnostic, err)
		}
	})

	t.Run("Should reject an unsupported Git version", func(t *testing.T) {
		t.Parallel()
		runner := &scriptedGitRunner{stdout: []byte("git version 2.30.1\n")}
		gate := NewCapabilityGateWithLookPath(runner, func(string) (string, error) { return "/usr/bin/git", nil })
		diagnostic, err := gate.Check(context.Background())
		if !errors.Is(err, ErrGitVersionUnsupported) || !strings.Contains(diagnostic.Message, "2.30.1") {
			t.Fatalf("Check() = (%#v, %v), want unsupported 2.30.1", diagnostic, err)
		}
	})

	t.Run("Should reject malformed Git version output deterministically", func(t *testing.T) {
		t.Parallel()
		for _, output := range []string{"", "git version 2", "git version two.45", "git version 2.x"} {
			t.Run("Should reject "+output, func(t *testing.T) {
				t.Parallel()
				runner := &scriptedGitRunner{stdout: []byte(output)}
				gate := NewCapabilityGateWithLookPath(runner, func(string) (string, error) {
					return "/usr/bin/git", nil
				})
				if _, err := gate.Check(context.Background()); !errors.Is(err, ErrGitVersionUnsupported) {
					t.Fatalf("Check(%q) error = %v, want unsupported", output, err)
				}
			})
		}
	})

	t.Run("Should cache a successful version probe", func(t *testing.T) {
		t.Parallel()
		runner := &scriptedGitRunner{stdout: []byte("git version 2.45.0\n")}
		gate := NewCapabilityGateWithLookPath(runner, func(string) (string, error) { return "/usr/bin/git", nil })
		for range 2 {
			diagnostic, err := gate.Check(context.Background())
			if err != nil || !diagnostic.Available {
				t.Fatalf("Check() = (%#v, %v), want available", diagnostic, err)
			}
		}
		if runner.callCount() != 1 {
			t.Fatalf("version probe calls = %d, want 1", runner.callCount())
		}
	})

	t.Run("Should retry a capability probe canceled by its caller", func(t *testing.T) {
		t.Parallel()
		runner := &scriptedGitRunner{stdout: []byte("git version 2.45.0\n")}
		gate := NewCapabilityGateWithLookPath(runner, func(string) (string, error) { return "/usr/bin/git", nil })
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := gate.Check(canceled); err == nil {
			t.Fatal("Check(canceled) error = nil")
		}
		diagnostic, err := gate.Check(context.Background())
		if err != nil || !diagnostic.Available || runner.callCount() != 2 {
			t.Fatalf(
				"Check(retry) = (%#v, %v), calls = %d; want available after two probes",
				diagnostic,
				err,
				runner.callCount(),
			)
		}
	})

	t.Run("Should preserve user environment and replace only unsafe Git process keys", func(t *testing.T) {
		t.Parallel()
		parent := []string{
			"HOME=/operator", "PATH=/bin", "GIT_CONFIG_GLOBAL=/operator/.gitconfig",
			"GIT_DIR=/tmp/admin", "GIT_WORK_TREE=/tmp/tree", "GIT_INDEX_FILE=/tmp/index",
			"GIT_TERMINAL_PROMPT=1", "GCM_INTERACTIVE=always",
		}
		got := gitEnvironment(parent)
		want := []string{
			"HOME=/operator", "PATH=/bin", "GIT_CONFIG_GLOBAL=/operator/.gitconfig",
			"GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("gitEnvironment() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should prevent claim and bound secrets from reaching Git subprocesses", func(t *testing.T) {
		t.Parallel()
		const secret = "git-bound-secret-123456"
		cleanup := diagnostics.RegisterRequiredSecret(secret)
		t.Cleanup(cleanup)
		got := gitEnvironment([]string{
			"PATH=/bin", "HOME=/operator", "GIT_CONFIG_GLOBAL=/operator/.gitconfig",
			"CLAIM_TOKEN=compozy_claim_" + secret, "BOUND_SECRET=" + secret,
			"SAFE_NAME=" + secret, "LANG=en_US.UTF-8",
		})
		joined := strings.Join(got, "\n")
		if strings.Contains(joined, secret) || strings.Contains(joined, "CLAIM_TOKEN") ||
			strings.Contains(joined, "BOUND_SECRET") || strings.Contains(joined, "SAFE_NAME") {
			t.Fatalf("git environment leaked secret material: %#v", got)
		}
		for _, want := range []string{
			"PATH=/bin", "HOME=/operator", "GIT_CONFIG_GLOBAL=/operator/.gitconfig", "LANG=en_US.UTF-8",
		} {
			if !strings.Contains(joined, want) {
				t.Fatalf("git environment = %#v, missing operator setting %q", got, want)
			}
		}
	})

	t.Run("Should execute the resolved Git binary and capture failures", func(t *testing.T) {
		t.Parallel()
		runner, err := NewRealGitRunner(2 * time.Second)
		if err != nil {
			t.Fatalf("NewRealGitRunner() error = %v", err)
		}
		stdout, stderr, err := runner.Run(context.Background(), "", "--version")
		if err != nil || !strings.HasPrefix(string(stdout), "git version ") || len(stderr) != 0 {
			t.Fatalf("Run(--version) = %q, %q, %v, want captured Git version", stdout, stderr, err)
		}

		failed := &RealGitRunner{executable: "/bin/sh", timeout: time.Second, environ: os.Environ}
		_, stderr, err = failed.Run(context.Background(), "", "-c", "printf failed >&2; exit 7")
		if err == nil || string(stderr) != "failed" || !strings.Contains(err.Error(), "failed") {
			t.Fatalf("Run(failure) stderr/error = %q / %v, want captured diagnostic", stderr, err)
		}

		var unavailable *RealGitRunner
		if _, _, err := unavailable.Run(context.Background(), "", "--version"); !errors.Is(err, ErrGitUnavailable) {
			t.Fatalf("nil Run() error = %v, want ErrGitUnavailable", err)
		}
	})

	t.Run("Should bound output captured by the real runner", func(t *testing.T) {
		t.Parallel()
		runner := &RealGitRunner{executable: "/bin/sh", timeout: time.Second, environ: os.Environ}
		stdout, stderr, err := runner.Run(
			context.Background(),
			"",
			"-c",
			"yes x | head -c 1100000; yes e | head -c 1100000 >&2",
		)
		if err != nil {
			t.Fatalf("Run(large output) error = %v", err)
		}
		if len(stdout) != maxGitOutputBytes || len(stderr) != maxGitOutputBytes {
			t.Fatalf(
				"Run(large output) captured stdout=%d stderr=%d, want %d each",
				len(stdout),
				len(stderr),
				maxGitOutputBytes,
			)
		}
	})

	t.Run("Should stream both output channels while preserving captured output", func(t *testing.T) {
		t.Parallel()
		runner := &RealGitRunner{executable: "/bin/sh", timeout: time.Second, environ: os.Environ}
		var mu sync.Mutex
		chunks := map[GitOutputStream]string{}
		stdout, stderr, err := runner.RunStreaming(
			context.Background(), "", func(output GitOutput) {
				mu.Lock()
				chunks[output.Stream] += string(output.Chunk)
				mu.Unlock()
			},
			"-c", "printf streamed-out; printf streamed-err >&2",
		)
		if err != nil || string(stdout) != "streamed-out" || string(stderr) != "streamed-err" {
			t.Fatalf("RunStreaming() = %q, %q, %v", stdout, stderr, err)
		}
		mu.Lock()
		defer mu.Unlock()
		if chunks[GitOutputStdout] != "streamed-out" || chunks[GitOutputStderr] != "streamed-err" {
			t.Fatalf("streamed chunks = %#v, want both complete channels", chunks)
		}
	})

	t.Run("Should kill and wait for a command that exceeds its timeout", func(t *testing.T) {
		t.Parallel()
		runner := &RealGitRunner{executable: "/bin/sh", timeout: 250 * time.Millisecond, environ: os.Environ}
		_, stderr, err := runner.Run(context.Background(), "", "-c", "echo timed-out >&2; sleep 5")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Run() error = %v, want context deadline exceeded", err)
		}
		if !strings.Contains(string(stderr), "timed-out") || !strings.Contains(err.Error(), "timed-out") {
			t.Fatalf("Run() stderr/error = %q / %v, want captured stderr", stderr, err)
		}
	})
}
