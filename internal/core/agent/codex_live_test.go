package agent

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

func TestLiveCodexModelAvailability(t *testing.T) {
	t.Run("Should create a real Codex ACP session with the requested model", func(t *testing.T) {
		modelName := strings.TrimSpace(os.Getenv("COMPOZY_LIVE_CODEX_MODEL"))
		if modelName == "" {
			t.Skip("set COMPOZY_LIVE_CODEX_MODEL to run the live Codex ACP model availability check")
		}

		reasoningEffort := strings.TrimSpace(os.Getenv("COMPOZY_LIVE_CODEX_REASONING_EFFORT"))
		if reasoningEffort == "" {
			reasoningEffort = "low"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		client, err := NewClient(ctx, ClientConfig{
			IDE:             model.IDECodex,
			Model:           modelName,
			ReasoningEffort: reasoningEffort,
			ShutdownTimeout: 5 * time.Second,
		})
		if err != nil {
			t.Fatalf("new codex client: %v", err)
		}
		t.Cleanup(func() {
			if err := client.Close(); err != nil {
				t.Fatalf("close codex client: %v", err)
			}
		})

		session, err := client.CreateSession(ctx, SessionRequest{
			Prompt:     []byte("Reply with exactly: compozy-model-ok"),
			WorkingDir: t.TempDir(),
			Model:      modelName,
		})
		if err != nil {
			t.Fatalf("create codex session with model %q: %v", modelName, err)
		}

		_ = collectSessionUpdates(t, session)
		if err := session.Err(); err != nil {
			t.Fatalf("codex session with model %q failed: %v", modelName, err)
		}
	})
}
