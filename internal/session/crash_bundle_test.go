package session

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/subprocess"
)

func TestPromptCrashBundleProcessEvidence(t *testing.T) {
	t.Run("Should stop the process before attaching the public event crash bundle", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			h := newHarness(t)
			sess := createSession(t, h)
			proc := h.driver.lastProcess()
			proc.handle.exitStatusFn = func() (subprocess.ExitStatus, bool) {
				select {
				case <-proc.done:
					return subprocess.ExitStatus{ExitCode: 23}, true
				default:
					return subprocess.ExitStatus{}, false
				}
			}
			stopStarted := make(chan struct{})
			releaseStop := make(chan struct{})
			h.driver.stopHook = func(proc *fakeProcess) error {
				close(stopStarted)
				<-releaseStop
				proc.finish(nil, "codex stderr tail")
				return nil
			}
			type barrierResult struct {
				event acp.AgentEvent
				fatal *promptPumpFatal
			}
			result := make(chan barrierResult, 1)
			go func() {
				event := acp.AgentEvent{
					Type:  acp.EventTypeError,
					Error: "peer disconnected before response",
					Failure: &store.SessionFailure{
						Kind:    store.FailureProcess,
						Summary: "ACP subprocess exited unexpectedly",
					},
				}
				fatal := &promptPumpFatal{}
				if !h.manager.prepareFatalPromptFailureEvent(t.Context(), sess, event, fatal) {
					result <- barrierResult{event: event, fatal: fatal}
					return
				}
				event = h.manager.attachPromptFailureDiagnostics(t.Context(), sess, event)
				fatal.capture(event.Failure, event.Error)
				result <- barrierResult{event: event, fatal: fatal}
			}()

			synctest.Wait()
			select {
			case <-stopStarted:
			default:
				t.Fatal("fatal prompt barrier did not start process stop")
			}
			select {
			case got := <-result:
				t.Fatalf("fatal prompt barrier returned before process completion: %#v", got.event)
			default:
			}

			close(releaseStop)
			synctest.Wait()
			got := <-result
			event := got.event
			if event.Failure == nil || event.Failure.CrashBundlePath == "" {
				t.Fatalf("prompt failure = %#v, want crash bundle path", event.Failure)
			}
			bundle := readCrashBundleDocument(t, event.Failure.CrashBundlePath)
			if bundle.Process == nil || bundle.Process.ExitCode == nil || *bundle.Process.ExitCode != 23 {
				t.Fatalf("crash bundle process = %#v, want exit code 23", bundle.Process)
			}
			if got, want := bundle.Stderr, "codex stderr tail"; got != want {
				t.Fatalf("crash bundle stderr = %q, want %q", got, want)
			}
			h.manager.finalizeSessionAfterFatalPromptFailure(t.Context(), sess, got.fatal)
		})
	})

	t.Run("Should serialize a Unix signal without a synthetic negative exit code", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() {
			reportSessionStop(t, h, sess.ID)
		})
		proc := h.driver.lastProcess()
		proc.handle.exitStatusFn = func() (subprocess.ExitStatus, bool) {
			return subprocess.ExitStatus{ExitCode: -1, Signal: "killed"}, true
		}

		path, err := h.manager.writeCrashBundle(
			sess,
			store.SessionFailure{Kind: store.FailureProcess, Summary: "runtime killed"},
			nil,
			"",
		)
		if err != nil {
			t.Fatalf("writeCrashBundle() error = %v", err)
		}
		bundle := readCrashBundleDocument(t, path)
		if bundle.Process == nil {
			t.Fatal("crash bundle process = nil, want signal evidence")
		}
		if bundle.Process.ExitCode != nil {
			t.Fatalf("crash bundle exit_code = %d, want omitted for signal exit", *bundle.Process.ExitCode)
		}
		if got, want := bundle.Process.Signal, "killed"; got != want {
			t.Fatalf("crash bundle signal = %q, want %q", got, want)
		}
	})
}

func readCrashBundleDocument(t *testing.T, path string) crashBundleDocument {
	t.Helper()

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	var document crashBundleDocument
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatalf("json.Unmarshal(crash bundle) error = %v", err)
	}
	return document
}

func TestCrashBundleFileName(t *testing.T) {
	t.Parallel()

	t.Run("ShouldPreserveTimestampWhenSessionIDIsTruncated", func(t *testing.T) {
		t.Parallel()

		first := time.Unix(0, 101).UTC()
		second := time.Unix(0, 202).UTC()
		sessionID := strings.Repeat("session-", 40)

		firstName := crashBundleFileName(sessionID, store.FailureProcess, first)
		secondName := crashBundleFileName(sessionID, store.FailureProcess, second)
		firstToken := strconv.FormatInt(first.UnixNano(), 10)
		secondToken := strconv.FormatInt(second.UnixNano(), 10)

		if firstName == secondName {
			t.Fatalf("crashBundleFileName() reused %q for distinct timestamps", firstName)
		}
		if !strings.Contains(firstName, firstToken) || !strings.Contains(secondName, secondToken) {
			t.Fatalf("crash bundle names = %q, %q; want timestamp suffixes preserved", firstName, secondName)
		}
		if len(strings.TrimSuffix(firstName, ".json")) > crashBundleNameMaxBytes ||
			len(strings.TrimSuffix(secondName, ".json")) > crashBundleNameMaxBytes {
			t.Fatalf("crash bundle names exceed max base bytes: %q, %q", firstName, secondName)
		}
	})
}
