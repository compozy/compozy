package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadStateReturnsNilWhenFileDoesNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yml")

	got, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil state, got %#v", got)
	}
}

func TestReadStateReturnsNilWhenYAMLIsCorrupted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yml")
	if err := writeFile(path, []byte("checked_for_update_at: ["), 0o644); err != nil {
		t.Fatalf("write corrupted state: %v", err)
	}

	got, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil state, got %#v", got)
	}
}

func TestReadStateWriteStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yml")
	want := &StateEntry{
		CheckedForUpdateAt: time.Date(2026, time.April, 7, 10, 30, 0, 0, time.UTC),
		LatestRelease: ReleaseInfo{
			Version:     "1.2.3",
			URL:         "https://github.com/compozy/compozy/releases/tag/v1.2.3",
			PublishedAt: time.Date(2026, time.April, 6, 9, 0, 0, 0, time.UTC),
		},
	}

	if err := WriteState(path, want); err != nil {
		t.Fatalf("WriteState returned error: %v", err)
	}

	got, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}
	if got == nil {
		t.Fatal("expected state entry, got nil")
	}
	if !got.CheckedForUpdateAt.Equal(want.CheckedForUpdateAt) {
		t.Fatalf("unexpected checked time: want %s, got %s", want.CheckedForUpdateAt, got.CheckedForUpdateAt)
	}
	if got.LatestRelease != want.LatestRelease {
		t.Fatalf("unexpected latest release: want %#v, got %#v", want.LatestRelease, got.LatestRelease)
	}
}

func TestStateFilePathUsesXDGConfigHomeWhenSet(t *testing.T) {
	xdgHome := filepath.Join(os.TempDir(), "compozy-xdg")
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	path, err := StateFilePath()
	if err != nil {
		t.Fatalf("StateFilePath returned error: %v", err)
	}

	want := filepath.Join(xdgHome, "compozy", "state.yml")
	if path != want {
		t.Fatalf("unexpected state file path: want %q, got %q", want, path)
	}
}

func TestStateFilePathFallsBackToHomeConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	homeDir := filepath.Join(os.TempDir(), "compozy-home")

	previous := osUserHomeDir
	osUserHomeDir = func() (string, error) {
		return homeDir, nil
	}
	t.Cleanup(func() {
		osUserHomeDir = previous
	})

	path, err := StateFilePath()
	if err != nil {
		t.Fatalf("StateFilePath returned error: %v", err)
	}

	want := filepath.Join(homeDir, ".config", "compozy", "state.yml")
	if path != want {
		t.Fatalf("unexpected state file path: want %q, got %q", want, path)
	}
}

func TestWriteStateRejectsEmptyPath(t *testing.T) {
	err := WriteState("", &StateEntry{})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestWriteStateRejectsNilEntry(t *testing.T) {
	err := WriteState(filepath.Join(t.TempDir(), "state.yml"), nil)
	if err == nil {
		t.Fatal("expected error for nil entry")
	}
}
