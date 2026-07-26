package soul

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestManagedSoulAuthoringServiceVerifyUnchangedSoul(t *testing.T) {
	t.Parallel()

	t.Run("Should detect conflicts when invalid SOUL content changes between mutation reads", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		agentPath := filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.AgentsDirName, "coder", "AGENT.md")
		writeTestFile(t, agentPath, "---\nname: coder\nprovider: codex\n---\nYou are coder.\n")

		soulPath := filepath.Join(filepath.Dir(agentPath), FileName)
		writeTestFile(t, soulPath, "---\nprovider: claude\n---\nFirst invalid body.\n")

		service := &ManagedSoulAuthoringService{}
		target, err := service.resolveTarget(context.Background(), AuthoringTarget{
			WorkspaceID:   "ws-authoring",
			WorkspaceRoot: workspaceRoot,
			AgentName:     "coder",
			AgentPath:     agentPath,
			Config:        testSoulConfig(),
			ConfigSource:  "test",
		})
		if err != nil {
			t.Fatalf("resolveTarget() error = %v", err)
		}

		current, err := service.currentSoulForMutation(context.Background(), target)
		if err != nil {
			t.Fatalf("currentSoulForMutation() error = %v", err)
		}
		if !current.resolved.Present || current.resolved.Valid {
			t.Fatalf("current.resolved = %#v, want present invalid state", current.resolved)
		}
		if current.resolved.Digest != "" {
			t.Fatalf(
				"current.resolved.Digest = %q, want empty digest for invalid current state",
				current.resolved.Digest,
			)
		}
		if current.compareToken == "" {
			t.Fatal("current.compareToken = empty, want invalid-state CAS token")
		}

		writeTestFile(t, soulPath, "---\nprovider: openai\n---\nSecond invalid body.\n")

		err = service.verifyUnchangedSoul(context.Background(), target, &current)
		if !errors.Is(err, ErrAuthoringConflict) {
			t.Fatalf("verifyUnchangedSoul() error = %v, want ErrAuthoringConflict", err)
		}
		var authoringErr *AuthoringError
		if !errors.As(err, &authoringErr) {
			t.Fatalf("verifyUnchangedSoul() error = %T %[1]v, want *AuthoringError", err)
		}
		if authoringErr.Code != diagnosticSoulConflict {
			t.Fatalf("AuthoringError.Code = %q, want %q", authoringErr.Code, diagnosticSoulConflict)
		}
	})
}
