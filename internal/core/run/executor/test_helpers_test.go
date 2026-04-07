package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"
	"github.com/compozy/compozy/internal/core/run/journal"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

type uiMsg = any

const (
	attemptStatusSuccess     = runshared.AttemptStatusSuccess
	attemptStatusFailure     = runshared.AttemptStatusFailure
	attemptStatusTimeout     = runshared.AttemptStatusTimeout
	attemptStatusCanceled    = runshared.AttemptStatusCanceled
	attemptStatusSetupFailed = runshared.AttemptStatusSetupFailed
)

func openRuntimeEventCapture(
	t *testing.T,
) (string, *journal.Journal, <-chan eventspkg.Event, func()) {
	t.Helper()

	workspaceRoot := t.TempDir()
	runArtifacts := model.NewRunArtifacts(workspaceRoot, "logging-test-run")
	if err := os.MkdirAll(filepath.Dir(runArtifacts.EventsPath), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	bus := eventspkg.New[eventspkg.Event](16)
	_, ch, unsubscribe := bus.Subscribe()
	runJournal, err := journal.Open(runArtifacts.EventsPath, bus, 16)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}

	cleanup := func() {
		t.Helper()
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runJournal.Close(closeCtx); err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("close journal: %v", err)
		}
		unsubscribe()
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	}

	return runArtifacts.RunID, runJournal, ch, cleanup
}

func collectRuntimeEvents(t *testing.T, ch <-chan eventspkg.Event, want int) []eventspkg.Event {
	t.Helper()

	got := make([]eventspkg.Event, 0, want)
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()

	for len(got) < want {
		select {
		case ev := <-ch:
			got = append(got, ev)
		case <-deadline.C:
			t.Fatalf("timed out waiting for %d runtime events, got %d", want, len(got))
		}
	}

	return got
}

func decodeRuntimeEventPayload(t *testing.T, ev eventspkg.Event, dst any) {
	t.Helper()

	if err := json.Unmarshal(ev.Payload, dst); err != nil {
		t.Fatalf("decode %s payload: %v", ev.Kind, err)
	}
}

func composeSessionPrompt(prompt []byte, systemPrompt string) []byte {
	basePrompt := append([]byte(nil), prompt...)
	if strings.TrimSpace(systemPrompt) == "" {
		return basePrompt
	}
	return []byte(strings.TrimSpace(systemPrompt) + "\n\n" + string(basePrompt))
}

func handleNilExecution(j *job, _ int, _ bool) jobAttemptResult {
	codeFileLabel := j.CodeFileLabel()
	failure := failInfo{
		CodeFile: codeFileLabel,
		ExitCode: -1,
		OutLog:   j.OutLog,
		ErrLog:   j.ErrLog,
		Err:      fmt.Errorf("failed to set up ACP session execution"),
	}
	return jobAttemptResult{Status: attemptStatusSetupFailed, ExitCode: -1, Failure: &failure}
}
