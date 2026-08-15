package attachments

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFilesystemAttachmentStoreRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("Should put open stat and delete one attachment", func(t *testing.T) {
		t.Parallel()

		store := openTestAttachmentStore(t, filepath.Join(t.TempDir(), "session-attachments"), testAttachmentRetention())
		payload := encodeTestPNG(t, 2, 2)

		ref, err := store.Put(t.Context(), "ws_a", "sess_1", "shot.png", payload)
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		if ref.ID == "" || ref.URI == "" || ref.SHA256 == "" || ref.Bytes != int64(len(payload)) {
			t.Fatalf("Put() ref = %#v, want a complete identity", ref)
		}
		if ref.MIMEType != MIMEImagePNG || ref.Kind != KindImage || ref.Width != 2 || ref.Height != 2 {
			t.Fatalf("Put() ref metadata = %#v", ref)
		}
		for _, path := range []string{
			store.contentPath("ws_a", "sess_1", ref.ID),
			metaPath(store.contentPath("ws_a", "sess_1", ref.ID)),
		} {
			info, statErr := os.Stat(path)
			if statErr != nil {
				t.Fatalf("Stat(%q) error = %v", path, statErr)
			}
			if got := info.Mode().Perm(); got != attachmentFileMode {
				t.Fatalf("mode(%q) = %o, want %o", path, got, attachmentFileMode)
			}
		}

		stat, err := store.Stat(t.Context(), "ws_a", "sess_1", ref.ID)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if stat != ref {
			t.Fatalf("Stat() = %#v, want %#v", stat, ref)
		}

		reader, opened, err := store.Open(t.Context(), "ws_a", "sess_1", ref.ID)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		body, err := io.ReadAll(reader)
		if closeErr := reader.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if opened != ref || !bytes.Equal(body, payload) {
			t.Fatalf("Open() = %#v %q, want %#v original bytes", opened, body, ref)
		}

		foreign, err := store.Stat(t.Context(), "ws_b", "sess_1", ref.ID)
		if !errors.Is(err, ErrNotFound) || foreign != (AttachmentRef{}) {
			t.Fatalf("Stat(foreign workspace) = %#v, %v, want not found", foreign, err)
		}

		if err := store.Delete(t.Context(), "ws_a", "sess_1", ref.ID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if _, err := store.Stat(t.Context(), "ws_a", "sess_1", ref.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Stat() after delete error = %v, want not found", err)
		}
	})

	t.Run("Should return the existing ref when the same digest is put again", func(t *testing.T) {
		t.Parallel()

		store := openTestAttachmentStore(t, filepath.Join(t.TempDir(), "session-attachments"), testAttachmentRetention())
		payload := []byte("same markdown body")

		first, err := store.Put(t.Context(), "ws_a", "sess_1", "notes.md", payload)
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}
		second, err := store.Put(t.Context(), "ws_a", "sess_1", "other.md", payload)
		if err != nil {
			t.Fatalf("Put(second) error = %v", err)
		}
		if first.ID != second.ID || first.SHA256 != second.SHA256 {
			t.Fatalf("Put(idempotent) = %#v, want %#v", second, first)
		}
		if first.Name != "notes.md" || second.Name != "notes.md" {
			t.Fatalf("Put(idempotent) names = %q %q, want original name", first.Name, second.Name)
		}
	})
}

func TestFilesystemAttachmentStorePathSanitization(t *testing.T) {
	t.Parallel()

	store := openTestAttachmentStore(t, filepath.Join(t.TempDir(), "session-attachments"), testAttachmentRetention())
	payload := []byte("plain notes")

	cases := []struct {
		name        string
		workspaceID string
		sessionID   string
		id          string
	}{
		{name: "Should reject a parent-directory workspace", workspaceID: "../escape", sessionID: "sess_1"},
		{name: "Should reject a slashed session id", workspaceID: "ws_a", sessionID: "sess/1"},
		{name: "Should reject a dotted traversal session", workspaceID: "ws_a", sessionID: ".."},
		{name: "Should reject an empty workspace", workspaceID: "", sessionID: "sess_1"},
		{name: "Should reject a traversal attachment id", workspaceID: "ws_a", sessionID: "sess_1", id: "../att_x"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.id == "" {
				_, err := store.Put(t.Context(), tc.workspaceID, tc.sessionID, "notes.txt", payload)
				if !errors.Is(err, ErrInvalidID) {
					t.Fatalf("Put() error = %v, want invalid id", err)
				}
				return
			}
			_, err := store.Stat(t.Context(), tc.workspaceID, tc.sessionID, tc.id)
			if !errors.Is(err, ErrInvalidID) {
				t.Fatalf("Stat() error = %v, want invalid id", err)
			}
		})
	}
}

func TestFilesystemAttachmentStoreSizeLimit(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a payload over the configured per-file ceiling", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join(t.TempDir(), "session-attachments")
		store, err := OpenFilesystemAttachmentStore(
			t.Context(),
			root,
			testAttachmentRetention(),
			StoreLimits{MaxFileBytes: 4, AllowedMIME: DefaultAllowedMIME},
		)
		if err != nil {
			t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
		}
		_, err = store.Put(t.Context(), "ws_a", "sess_1", "notes.txt", []byte("hello"))
		if !errors.Is(err, ErrTooLarge) {
			t.Fatalf("Put() error = %v, want too large", err)
		}
	})

	t.Run("Should reject a payload larger than the total retention budget", func(t *testing.T) {
		t.Parallel()

		retention := testAttachmentRetention()
		retention.MaxBytes = 4
		store, err := OpenFilesystemAttachmentStore(
			t.Context(),
			filepath.Join(t.TempDir(), "session-attachments"),
			retention,
			StoreLimits{MaxFileBytes: 8, AllowedMIME: DefaultAllowedMIME},
		)
		if err != nil {
			t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
		}
		_, err = store.Put(t.Context(), "ws_a", "sess_1", "notes.txt", []byte("hello"))
		if !errors.Is(err, ErrTooLarge) {
			t.Fatalf("Put() error = %v, want too large", err)
		}
	})

	t.Run("Should reject MIME outside the configured allowlist", func(t *testing.T) {
		t.Parallel()

		store, err := OpenFilesystemAttachmentStore(
			t.Context(),
			filepath.Join(t.TempDir(), "session-attachments"),
			testAttachmentRetention(),
			StoreLimits{MaxFileBytes: 1024, AllowedMIME: []string{MIMETextPlain}},
		)
		if err != nil {
			t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
		}
		_, err = store.Put(t.Context(), "ws_a", "sess_1", "notes.md", []byte("# notes"))
		if !errors.Is(err, ErrUnsupportedMIME) {
			t.Fatalf("Put() error = %v, want unsupported MIME", err)
		}
	})
}

func TestOpenFilesystemAttachmentStoreValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		root      string
		retention AttachmentRetention
		limits    StoreLimits
	}{
		{
			name:      "Should reject an empty root",
			retention: testAttachmentRetention(),
			limits:    testAttachmentLimits(),
		},
		{
			name: "Should reject invalid retention",
			root: filepath.Join(t.TempDir(), "invalid-retention"),
			retention: AttachmentRetention{
				MaxCount: 0,
				MaxBytes: 1024,
				MaxAge:   time.Hour,
			},
			limits: testAttachmentLimits(),
		},
		{
			name:      "Should reject a non-positive file limit",
			root:      filepath.Join(t.TempDir(), "invalid-file-limit"),
			retention: testAttachmentRetention(),
			limits:    StoreLimits{AllowedMIME: DefaultAllowedMIME},
		},
		{
			name:      "Should reject an empty MIME allowlist",
			root:      filepath.Join(t.TempDir(), "empty-allowlist"),
			retention: testAttachmentRetention(),
			limits:    StoreLimits{MaxFileBytes: 1024},
		},
		{
			name:      "Should reject a MIME outside the v1 allowlist",
			root:      filepath.Join(t.TempDir(), "unsupported-allowlist"),
			retention: testAttachmentRetention(),
			limits: StoreLimits{
				MaxFileBytes: 1024,
				AllowedMIME:  []string{"image/gif"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := OpenFilesystemAttachmentStore(
				t.Context(),
				tc.root,
				tc.retention,
				tc.limits,
			); err == nil {
				t.Fatal("OpenFilesystemAttachmentStore() error = nil, want validation error")
			}
		})
	}
}

func TestFilesystemAttachmentStoreRetention(t *testing.T) {
	t.Parallel()

	t.Run("Should evict the oldest attachment by count", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 14, 4, 0, 0, 0, time.UTC)
		retention := testAttachmentRetention()
		retention.MaxCount = 2
		store, err := OpenFilesystemAttachmentStore(
			t.Context(),
			filepath.Join(t.TempDir(), "session-attachments"),
			retention,
			testAttachmentLimits(),
			withAttachmentStoreNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
		}
		first, err := store.Put(t.Context(), "ws_a", "sess_1", "a.txt", []byte("first"))
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}
		now = now.Add(time.Second)
		second, err := store.Put(t.Context(), "ws_a", "sess_1", "b.txt", []byte("second"))
		if err != nil {
			t.Fatalf("Put(second) error = %v", err)
		}
		now = now.Add(time.Second)
		third, err := store.Put(t.Context(), "ws_a", "sess_1", "c.txt", []byte("third"))
		if err != nil {
			t.Fatalf("Put(third) error = %v", err)
		}
		assertAttachmentMissing(t, store, "ws_a", "sess_1", first.ID)
		assertAttachmentPresent(t, store, "ws_a", "sess_1", second.ID)
		assertAttachmentPresent(t, store, "ws_a", "sess_1", third.ID)
	})

	t.Run("Should enforce bytes and age across session directories", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 14, 4, 0, 0, 0, time.UTC)
		retention := testAttachmentRetention()
		retention.MaxBytes = 9
		retention.MaxAge = time.Hour
		store, err := OpenFilesystemAttachmentStore(
			t.Context(),
			filepath.Join(t.TempDir(), "session-attachments"),
			retention,
			testAttachmentLimits(),
			withAttachmentStoreNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
		}
		old, err := store.Put(t.Context(), "ws_a", "sess_1", "old.txt", []byte("12345"))
		if err != nil {
			t.Fatalf("Put(old) error = %v", err)
		}
		now = now.Add(time.Second)
		current, err := store.Put(t.Context(), "ws_b", "sess_2", "new.txt", []byte("67890"))
		if err != nil {
			t.Fatalf("Put(current) error = %v", err)
		}
		assertAttachmentMissing(t, store, "ws_a", "sess_1", old.ID)
		assertAttachmentPresent(t, store, "ws_b", "sess_2", current.ID)

		now = now.Add(time.Hour + time.Second)
		assertAttachmentMissing(t, store, "ws_b", "sess_2", current.ID)
	})
}

func TestFilesystemAttachmentStoreFailureCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should remove published files when the writer reports failure", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join(t.TempDir(), "session-attachments")
		store, err := OpenFilesystemAttachmentStore(
			t.Context(),
			root,
			testAttachmentRetention(),
			testAttachmentLimits(),
			withAttachmentStoreWriteFile(func(path string, content []byte, perm os.FileMode) error {
				if writeErr := os.WriteFile(path, content, perm); writeErr != nil {
					return writeErr
				}
				return errors.New("injected sync failure")
			}),
		)
		if err != nil {
			t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
		}
		_, err = store.Put(t.Context(), "ws_a", "sess_1", "notes.txt", []byte("payload"))
		if !errors.Is(err, ErrPersistence) || !strings.Contains(err.Error(), "injected sync failure") {
			t.Fatalf("Put() error = %v, want typed injected persistence failure", err)
		}
		matches, globErr := filepath.Glob(filepath.Join(root, "*", "*", "att_*"))
		if globErr != nil {
			t.Fatalf("Glob() error = %v", globErr)
		}
		if len(matches) != 0 {
			t.Fatalf("published attachments after failure = %#v, want none", matches)
		}
	})
}

func TestFilesystemAttachmentStoreMetadataIntegrity(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a sidecar whose byte count differs from content", func(t *testing.T) {
		t.Parallel()

		store := openTestAttachmentStore(t, filepath.Join(t.TempDir(), "session-attachments"), testAttachmentRetention())
		ref, err := store.Put(t.Context(), "ws_a", "sess_1", "notes.txt", []byte("payload"))
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		sidecarPath := metaPath(store.contentPath("ws_a", "sess_1", ref.ID))
		encoded, err := os.ReadFile(sidecarPath)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		var meta attachmentMeta
		if err := json.Unmarshal(encoded, &meta); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		meta.Bytes++
		encoded, err = json.Marshal(meta)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if err := os.WriteFile(sidecarPath, encoded, attachmentFileMode); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if _, err := store.Stat(t.Context(), "ws_a", "sess_1", ref.ID); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("Stat() error = %v, want corrupt", err)
		}
	})

	t.Run("Should reject a symlink sidecar", func(t *testing.T) {
		t.Parallel()

		store := openTestAttachmentStore(t, filepath.Join(t.TempDir(), "session-attachments"), testAttachmentRetention())
		ref, err := store.Put(t.Context(), "ws_a", "sess_1", "notes.txt", []byte("payload"))
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		sidecarPath := metaPath(store.contentPath("ws_a", "sess_1", ref.ID))
		targetPath := filepath.Join(t.TempDir(), "outside.json")
		if err := os.WriteFile(targetPath, []byte("{}"), attachmentFileMode); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if err := os.Remove(sidecarPath); err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
		if err := os.Symlink(targetPath, sidecarPath); err != nil {
			t.Fatalf("Symlink() error = %v", err)
		}
		if _, err := store.Stat(t.Context(), "ws_a", "sess_1", ref.ID); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("Stat() error = %v, want corrupt", err)
		}
	})
}

func openTestAttachmentStore(
	t *testing.T,
	root string,
	retention AttachmentRetention,
) *FilesystemAttachmentStore {
	t.Helper()
	store, err := OpenFilesystemAttachmentStore(t.Context(), root, retention, testAttachmentLimits())
	if err != nil {
		t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
	}
	return store
}

func testAttachmentRetention() AttachmentRetention {
	return AttachmentRetention{MaxCount: 20, MaxBytes: 1 << 20, MaxAge: 24 * time.Hour}
}

func testAttachmentLimits() StoreLimits {
	return StoreLimits{MaxFileBytes: 1 << 20, AllowedMIME: DefaultAllowedMIME}
}

func assertAttachmentMissing(
	t *testing.T,
	store Store,
	workspaceID string,
	sessionID string,
	id string,
) {
	t.Helper()
	_, err := store.Stat(t.Context(), workspaceID, sessionID, id)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stat(%q) error = %v, want not found", id, err)
	}
}

func assertAttachmentPresent(
	t *testing.T,
	store Store,
	workspaceID string,
	sessionID string,
	id string,
) {
	t.Helper()
	if _, err := store.Stat(t.Context(), workspaceID, sessionID, id); err != nil {
		t.Fatalf("Stat(%q) error = %v, want retained attachment", id, err)
	}
}
