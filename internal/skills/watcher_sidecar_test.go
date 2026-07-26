package skills

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherSidecarSnapshotsContract(t *testing.T) {
	t.Parallel()

	t.Run("Should keep registry seeded sidecars in the watcher baseline", func(t *testing.T) {
		t.Parallel()

		_, watcher := newWatcherWithSidecarSkillContract(t)
		refreshes := 0
		watcher.SetAfterRefresh(func(context.Context) error {
			refreshes++
			return nil
		})

		if err := watcher.pollOnce(context.Background()); err != nil {
			t.Fatalf("pollOnce() error = %v", err)
		}
		if refreshes != 0 {
			t.Fatalf("watcher refreshes after unchanged sidecar baseline = %d, want 0", refreshes)
		}
	})

	t.Run("Should refresh global registry after a sidecar-only change", func(t *testing.T) {
		t.Parallel()

		mcpPath, watcher := newWatcherWithSidecarSkillContract(t)
		if err := watcher.pollOnce(context.Background()); err != nil {
			t.Fatalf("initial pollOnce() error = %v", err)
		}

		writeSkillMCPSidecar(t, filepath.Dir(mcpPath), `{
  "mcpServers": {
    "sidecar": {
      "command": "version-two-with-larger-content"
    }
  }
}`)

		refreshes := 0
		watcher.SetAfterRefresh(func(context.Context) error {
			refreshes++
			return nil
		})
		if err := watcher.pollOnce(context.Background()); err != nil {
			t.Fatalf("sidecar pollOnce() error = %v", err)
		}
		if refreshes != 1 {
			t.Fatalf("watcher refreshes after sidecar-only change = %d, want 1", refreshes)
		}
	})
}

func newWatcherWithSidecarSkillContract(t *testing.T) (string, *Watcher) {
	t.Helper()

	root := t.TempDir()
	userDir := filepath.Join(root, "user")
	skillDir := filepath.Join(userDir, "sidecar-watch")
	writeSkillFile(
		t,
		userDir,
		filepath.Join("sidecar-watch", skillFileName),
		skillWithDescription("sidecar-watch", "Sidecar watch skill"),
	)
	mcpPath := writeSkillMCPSidecar(t, skillDir, `{
  "mcpServers": {
    "sidecar": {
      "command": "version-one"
    }
  }
}`)

	registry := newTestRegistry(t, RegistryConfig{
		UserSkillsDir: userDir,
	})
	if err := registry.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	watcher := NewWatcher(registry, time.Millisecond)
	watcher.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return mcpPath, watcher
}
