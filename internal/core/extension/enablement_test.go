package extensions

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestEnablementStoreLoadsWorkspaceStateAcrossCanonicalRootAliases(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	legacyRoot := filepath.Clean("/Users/pedronauck/dev/compozy/agh2")
	canonicalRoot := filepath.Clean("/Users/pedronauck/Dev/compozy/agh2")
	store := &EnablementStore{
		homeDir: homeDir,
		normalizeWorkspaceRoot: func(root string) (string, error) {
			switch filepath.Clean(root) {
			case legacyRoot, canonicalRoot:
				return canonicalRoot, nil
			default:
				return filepath.Clean(root), nil
			}
		},
	}

	writeWorkspaceEnablementState(t, homeDir, workspaceEnablementRecord{
		Workspaces: map[string]map[string]bool{
			legacyRoot: {
				"cy-qa-workflow": true,
			},
		},
	})

	enabled, err := store.Enabled(context.Background(), Ref{
		Name:          "cy-qa-workflow",
		Source:        SourceWorkspace,
		WorkspaceRoot: canonicalRoot,
	})
	if err != nil {
		t.Fatalf("Enabled() error = %v", err)
	}
	if !enabled {
		t.Fatal("Enabled() = false, want true for canonical alias of persisted workspace root")
	}
}

func TestCanonicalizeExistingPathCaseWithUsesOnDiskNames(t *testing.T) {
	t.Parallel()

	root := extensionTestAbsoluteRoot(t)
	usersDir := filepath.Join(root, "Users")
	homeDir := filepath.Join(usersDir, "pedronauck")
	devDir := filepath.Join(homeDir, "Dev")
	compozyDir := filepath.Join(devDir, "compozy")
	want := filepath.Join(compozyDir, "agh2")
	input := filepath.Join(homeDir, "dev", "compozy", "agh2")

	dirs := map[string][]os.DirEntry{
		root:       {extensionFakeDirEntry{name: "Users"}},
		usersDir:   {extensionFakeDirEntry{name: "pedronauck"}},
		homeDir:    {extensionFakeDirEntry{name: "Dev"}},
		devDir:     {extensionFakeDirEntry{name: "compozy"}},
		compozyDir: {extensionFakeDirEntry{name: "agh2"}},
	}

	got, err := canonicalizeExistingPathCaseWith(input, func(path string) ([]os.DirEntry, error) {
		entries, ok := dirs[path]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return entries, nil
	})
	if err != nil {
		t.Fatalf("canonicalizeExistingPathCaseWith() error = %v", err)
	}
	if got != want {
		t.Fatalf("canonicalizeExistingPathCaseWith() = %q, want %q", got, want)
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

func writeWorkspaceEnablementState(t *testing.T, homeDir string, record workspaceEnablementRecord) {
	t.Helper()

	path := filepath.Join(homeDir, ".compozy", "state", workspaceEnablementStateFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	payload, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent(workspace enablement): %v", err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func extensionTestAbsoluteRoot(t *testing.T) string {
	t.Helper()

	root := string(filepath.Separator)
	if volume := filepath.VolumeName(t.TempDir()); volume != "" {
		root = volume + string(filepath.Separator)
	}
	return root
}

type extensionFakeDirEntry struct {
	name string
}

func (e extensionFakeDirEntry) Name() string               { return e.name }
func (e extensionFakeDirEntry) IsDir() bool                { return true }
func (e extensionFakeDirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (e extensionFakeDirEntry) Info() (fs.FileInfo, error) { return extensionFakeFileInfo(e), nil }

type extensionFakeFileInfo struct {
	name string
}

func (i extensionFakeFileInfo) Name() string       { return i.name }
func (i extensionFakeFileInfo) Size() int64        { return 0 }
func (i extensionFakeFileInfo) Mode() fs.FileMode  { return fs.ModeDir }
func (i extensionFakeFileInfo) ModTime() time.Time { return time.Time{} }
func (i extensionFakeFileInfo) IsDir() bool        { return true }
func (i extensionFakeFileInfo) Sys() any           { return nil }
