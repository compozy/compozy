package agent

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

func TestLiveOMPNewAndResumeSession(t *testing.T) {
	if strings.TrimSpace(os.Getenv("COMPOZY_LIVE_OMP")) != "1" {
		t.Skip("set COMPOZY_LIVE_OMP=1 to run the live OMP new/resume smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	client, err := NewClient(ctx, ClientConfig{
		IDE:             model.IDEOMP,
		Model:           model.DefaultOMPModel,
		ReasoningEffort: "medium",
		ShutdownTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("new OMP client: %v", err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = client.Close()
		}
	})

	workspace := t.TempDir()
	const firstMarker = "COMPOZY_OMP_NEW_OK"
	created, err := client.CreateSession(ctx, SessionRequest{
		Prompt:     []byte("Reply with exactly: " + firstMarker),
		WorkingDir: workspace,
		Model:      model.DefaultOMPModel,
	})
	if err != nil {
		t.Fatalf("create live OMP session: %v", err)
	}
	if got := liveOMPResponseText(t, created); got != firstMarker {
		t.Fatalf("live OMP new-session response = %q, want %q", got, firstMarker)
	}
	if !client.SupportsLoadSession() {
		t.Fatal("live OMP did not advertise session/load support")
	}

	const secondMarker = "COMPOZY_OMP_RESUME_OK"
	resumed, err := client.ResumeSession(ctx, ResumeSessionRequest{
		SessionID:  created.Identity().ACPSessionID,
		Prompt:     []byte("Reply with exactly: " + secondMarker),
		WorkingDir: workspace,
		Model:      model.DefaultOMPModel,
	})
	if err != nil {
		t.Fatalf("resume live OMP session: %v", err)
	}
	if got := liveOMPResponseText(t, resumed); got != secondMarker {
		t.Fatalf("live OMP resumed response = %q, want %q", got, secondMarker)
	}
	if !resumed.Identity().Resumed || resumed.Identity().ACPSessionID != created.Identity().ACPSessionID {
		t.Fatalf("live OMP resumed identity = %#v", resumed.Identity())
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close live OMP client: %v", err)
	}
	closed = true
}

func liveOMPResponseText(t *testing.T, session Session) string {
	t.Helper()

	updates := collectSessionUpdates(t, session)
	if err := session.Err(); err != nil {
		t.Fatalf("live OMP session failed: %v", err)
	}
	var output strings.Builder
	for _, block := range flattenBlocks(updates) {
		if block.Type != model.BlockText {
			continue
		}
		text, err := block.AsText()
		if err != nil {
			t.Fatalf("decode live OMP response: %v", err)
		}
		output.WriteString(text.Text)
	}
	return strings.TrimSpace(output.String())
}
