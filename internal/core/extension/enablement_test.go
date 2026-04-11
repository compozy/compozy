package extensions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEnablementStoreDefaults(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()

	store, err := NewEnablementStore(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("NewEnablementStore() error = %v", err)
	}

	testCases := []struct {
		name    string
		ref     Ref
		enabled bool
	}{
		{
			name: "bundled",
			ref: Ref{
				Name:   "bundled-ext",
				Source: SourceBundled,
			},
			enabled: true,
		},
		{
			name: "user",
			ref: Ref{
				Name:   "user-ext",
				Source: SourceUser,
			},
			enabled: false,
		},
		{
			name: "workspace",
			ref: Ref{
				Name:          "workspace-ext",
				Source:        SourceWorkspace,
				WorkspaceRoot: workspaceRoot,
			},
			enabled: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			enabled, err := store.Enabled(context.Background(), tc.ref)
			if err != nil {
				t.Fatalf("Enabled() error = %v", err)
			}
			if enabled != tc.enabled {
				t.Fatalf("Enabled() = %t, want %t", enabled, tc.enabled)
			}
		})
	}
}

func TestEnablementStorePersistsRoundTrip(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()

	testCases := []struct {
		name      string
		ref       Ref
		statePath string
	}{
		{
			name: "user",
			ref: Ref{
				Name:   "user-ext",
				Source: SourceUser,
			},
			statePath: filepath.Join(homeDir, ".compozy", "extensions", "user-ext", userEnablementStateFileName),
		},
		{
			name: "workspace",
			ref: Ref{
				Name:          "workspace-ext",
				Source:        SourceWorkspace,
				WorkspaceRoot: workspaceRoot,
			},
			statePath: filepath.Join(homeDir, ".compozy", "state", workspaceEnablementStateFileName),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store, err := NewEnablementStore(context.Background(), homeDir)
			if err != nil {
				t.Fatalf("NewEnablementStore() error = %v", err)
			}
			if err := store.Enable(context.Background(), tc.ref); err != nil {
				t.Fatalf("Enable() error = %v", err)
			}

			reloadedStore, err := NewEnablementStore(context.Background(), homeDir)
			if err != nil {
				t.Fatalf("NewEnablementStore() reload error = %v", err)
			}
			enabled, err := reloadedStore.Enabled(context.Background(), tc.ref)
			if err != nil {
				t.Fatalf("Enabled() reload error = %v", err)
			}
			if !enabled {
				t.Fatal("Enabled() after enable = false, want true")
			}

			if err := reloadedStore.Disable(context.Background(), tc.ref); err != nil {
				t.Fatalf("Disable() error = %v", err)
			}

			finalStore, err := NewEnablementStore(context.Background(), homeDir)
			if err != nil {
				t.Fatalf("NewEnablementStore() final error = %v", err)
			}
			enabled, err = finalStore.Enabled(context.Background(), tc.ref)
			if err != nil {
				t.Fatalf("Enabled() final error = %v", err)
			}
			if enabled {
				t.Fatal("Enabled() after disable = true, want false")
			}

			if _, err := os.Stat(tc.statePath); err != nil {
				t.Fatalf("os.Stat(%q) error = %v, want persisted state file", tc.statePath, err)
			}
		})
	}
}

func TestEnablementStoreRejectsBundledMutations(t *testing.T) {
	t.Parallel()

	store, err := NewEnablementStore(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("NewEnablementStore() error = %v", err)
	}

	err = store.Disable(context.Background(), Ref{
		Name:   "bundled-ext",
		Source: SourceBundled,
	})
	if err == nil {
		t.Fatal("Disable() error = nil, want bundled mutation failure")
	}
}

func TestNewEnablementStoreResolvesHomeDir(t *testing.T) {
	homeDir := t.TempDir()

	previous := osUserHomeDir
	osUserHomeDir = func() (string, error) {
		return homeDir, nil
	}
	t.Cleanup(func() {
		osUserHomeDir = previous
	})

	store, err := NewEnablementStore(context.Background(), "")
	if err != nil {
		t.Fatalf("NewEnablementStore() error = %v", err)
	}
	if store.homeDir != homeDir {
		t.Fatalf("homeDir = %q, want %q", store.homeDir, homeDir)
	}
}

func TestEnablementStoreRejectsInvalidReferences(t *testing.T) {
	t.Parallel()

	store, err := NewEnablementStore(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("NewEnablementStore() error = %v", err)
	}

	testCases := []struct {
		name string
		ref  Ref
	}{
		{
			name: "empty name",
			ref: Ref{
				Source: SourceUser,
			},
		},
		{
			name: "workspace missing root",
			ref: Ref{
				Name:   "workspace-ext",
				Source: SourceWorkspace,
			},
		},
		{
			name: "unsupported source",
			ref: Ref{
				Name:   "weird-ext",
				Source: Source("other"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := store.Enabled(context.Background(), tc.ref); err == nil {
				t.Fatal("Enabled() error = nil, want invalid reference failure")
			}
		})
	}
}

func TestEnablementStoreRejectsCorruptState(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()

	store, err := NewEnablementStore(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("NewEnablementStore() error = %v", err)
	}

	writeCorruptStateFile(t, filepath.Join(homeDir, ".compozy", "extensions", "user-ext", userEnablementStateFileName))
	if _, err := store.Enabled(context.Background(), Ref{Name: "user-ext", Source: SourceUser}); err == nil {
		t.Fatal("Enabled() user error = nil, want corrupt state failure")
	}

	writeCorruptStateFile(t, filepath.Join(homeDir, ".compozy", "state", workspaceEnablementStateFileName))
	if _, err := store.Enabled(context.Background(), Ref{
		Name:          "workspace-ext",
		Source:        SourceWorkspace,
		WorkspaceRoot: workspaceRoot,
	}); err == nil {
		t.Fatal("Enabled() workspace error = nil, want corrupt state failure")
	}
}

func writeCorruptStateFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
