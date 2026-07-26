package tools

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFilesystemToolArtifactStoreRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve exact bytes across reopen and isolate workspace reads", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join(t.TempDir(), "tool-artifacts")
		retention := testToolArtifactRetention()
		store := openTestToolArtifactStore(t, root, retention)
		content := []byte(`{"version":"agh.tool-result/v1","tail":"D6-TAIL"}`)

		ref, err := store.Put(t.Context(), "workspace-a", content)
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		if ref.URI == "" || ref.SHA256 == "" || ref.Bytes != int64(len(content)) {
			t.Fatalf("Put() ref = %#v, want complete opaque reference", ref)
		}
		foreign, err := store.ReadPage(t.Context(), "workspace-b", ref.URI, 0, 8)
		if !errors.Is(err, ErrToolArtifactNotFound) || foreign != (ToolArtifactPage{}) {
			t.Fatalf("ReadPage(foreign) = %#v, %v, want not found", foreign, err)
		}

		reopened := openTestToolArtifactStore(t, root, retention)
		var restored []byte
		for offset := int64(0); ; {
			page, readErr := reopened.ReadPage(t.Context(), "workspace-a", ref.URI, offset, 7)
			if readErr != nil {
				t.Fatalf("ReadPage(offset=%d) error = %v", offset, readErr)
			}
			decoded, decodeErr := base64.StdEncoding.DecodeString(page.DataBase64)
			if decodeErr != nil {
				t.Fatalf("DecodeString() error = %v", decodeErr)
			}
			restored = append(restored, decoded...)
			if page.EOF {
				break
			}
			offset = page.NextOffset
		}
		if !bytes.Equal(restored, content) {
			t.Fatalf("restored bytes = %q, want %q", restored, content)
		}
	})

	t.Run("Should reject invalid ranges without reading another workspace", func(t *testing.T) {
		t.Parallel()

		store := openTestToolArtifactStore(t, filepath.Join(t.TempDir(), "tool-artifacts"), testToolArtifactRetention())
		ref, err := store.Put(t.Context(), "workspace-a", []byte("content"))
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		_, err = store.ReadPage(t.Context(), "workspace-a", ref.URI, 8, 1)
		if !errors.Is(err, ErrToolArtifactInvalidRange) {
			t.Fatalf("ReadPage(offset beyond end) error = %v, want invalid range", err)
		}
		_, err = store.ReadPage(t.Context(), "workspace-a", ref.URI, 0, MaxToolArtifactPageBytes+1)
		if !errors.Is(err, ErrToolArtifactInvalidRange) {
			t.Fatalf("ReadPage(oversized limit) error = %v, want invalid range", err)
		}
	})
}

func TestFilesystemToolArtifactStoreRetention(t *testing.T) {
	t.Parallel()

	t.Run("Should evict the oldest artifact by count with deterministic ties", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.July, 19, 4, 0, 0, 0, time.UTC)
		root := filepath.Join(t.TempDir(), "tool-artifacts")
		retention := testToolArtifactRetention()
		retention.MaxCount = 2
		store, err := OpenFilesystemToolArtifactStore(
			t.Context(),
			root,
			retention,
			withArtifactStoreNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("OpenFilesystemToolArtifactStore() error = %v", err)
		}
		first, err := store.Put(t.Context(), "workspace-a", []byte("first"))
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}
		now = now.Add(time.Second)
		second, err := store.Put(t.Context(), "workspace-a", []byte("second"))
		if err != nil {
			t.Fatalf("Put(second) error = %v", err)
		}
		now = now.Add(time.Second)
		third, err := store.Put(t.Context(), "workspace-a", []byte("third"))
		if err != nil {
			t.Fatalf("Put(third) error = %v", err)
		}
		assertToolArtifactMissing(t, store, "workspace-a", first.URI)
		assertToolArtifactPresent(t, store, "workspace-a", second.URI)
		assertToolArtifactPresent(t, store, "workspace-a", third.URI)
	})

	t.Run("Should enforce bytes and age across workspace directories", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.July, 19, 4, 0, 0, 0, time.UTC)
		retention := testToolArtifactRetention()
		retention.MaxBytes = 9
		retention.MaxAge = time.Hour
		store, err := OpenFilesystemToolArtifactStore(
			t.Context(),
			filepath.Join(t.TempDir(), "tool-artifacts"),
			retention,
			withArtifactStoreNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("OpenFilesystemToolArtifactStore() error = %v", err)
		}
		old, err := store.Put(t.Context(), "workspace-a", []byte("12345"))
		if err != nil {
			t.Fatalf("Put(old) error = %v", err)
		}
		now = now.Add(time.Second)
		current, err := store.Put(t.Context(), "workspace-b", []byte("67890"))
		if err != nil {
			t.Fatalf("Put(current) error = %v", err)
		}
		assertToolArtifactMissing(t, store, "workspace-a", old.URI)
		assertToolArtifactPresent(t, store, "workspace-b", current.URI)

		now = now.Add(time.Hour + time.Second)
		assertToolArtifactMissing(t, store, "workspace-b", current.URI)
	})
}

func TestFilesystemToolArtifactStoreFailureCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should remove a published file when the writer reports failure", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join(t.TempDir(), "tool-artifacts")
		store, err := OpenFilesystemToolArtifactStore(
			t.Context(),
			root,
			testToolArtifactRetention(),
			withArtifactStoreWriteFile(func(path string, content []byte, perm os.FileMode) error {
				if writeErr := os.WriteFile(path, content, perm); writeErr != nil {
					return writeErr
				}
				return errors.New("injected sync failure")
			}),
		)
		if err != nil {
			t.Fatalf("OpenFilesystemToolArtifactStore() error = %v", err)
		}
		_, err = store.Put(t.Context(), "workspace-a", []byte("oversized result"))
		if !errors.Is(err, ErrToolArtifactPersistence) || !strings.Contains(err.Error(), "injected sync failure") {
			t.Fatalf("Put() error = %v, want typed injected persistence failure", err)
		}
		matches, globErr := filepath.Glob(filepath.Join(root, "*", "*.json"))
		if globErr != nil {
			t.Fatalf("Glob() error = %v", globErr)
		}
		if len(matches) != 0 {
			t.Fatalf("published artifacts after failure = %#v, want none", matches)
		}
	})
}

func openTestToolArtifactStore(
	t *testing.T,
	root string,
	retention ToolArtifactRetention,
) *FilesystemToolArtifactStore {
	t.Helper()
	store, err := OpenFilesystemToolArtifactStore(t.Context(), root, retention)
	if err != nil {
		t.Fatalf("OpenFilesystemToolArtifactStore() error = %v", err)
	}
	return store
}

func testToolArtifactRetention() ToolArtifactRetention {
	return ToolArtifactRetention{MaxCount: 20, MaxBytes: 1 << 20, MaxAge: 24 * time.Hour}
}

func assertToolArtifactMissing(
	t *testing.T,
	store ToolArtifactStore,
	workspaceID string,
	uri string,
) {
	t.Helper()
	_, err := store.ReadPage(t.Context(), workspaceID, uri, 0, 1)
	if !errors.Is(err, ErrToolArtifactNotFound) {
		t.Fatalf("ReadPage(%q) error = %v, want not found", uri, err)
	}
}

func assertToolArtifactPresent(
	t *testing.T,
	store ToolArtifactStore,
	workspaceID string,
	uri string,
) {
	t.Helper()
	if _, err := store.ReadPage(t.Context(), workspaceID, uri, 0, 1); err != nil {
		t.Fatalf("ReadPage(%q) error = %v, want retained artifact", uri, err)
	}
}

func decodeToolArtifactPage(page ToolArtifactPage) ([]byte, error) {
	return base64.StdEncoding.DecodeString(page.DataBase64)
}
