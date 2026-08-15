package attachments

import (
	"bytes"
	"context"
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

		store := openTestAttachmentStore(
			t,
			filepath.Join(t.TempDir(), "session-attachments"),
			testAttachmentRetention(),
		)
		payload := encodeTestPNG(t, 2, 2)

		ref, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "shot.png", Data: payload,
		})
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

		store := openTestAttachmentStore(
			t,
			filepath.Join(t.TempDir(), "session-attachments"),
			testAttachmentRetention(),
		)
		payload := []byte("same markdown body")

		first, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "notes.md", Data: payload,
		})
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}
		second, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "other.md", Data: payload,
		})
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
				_, err := store.Put(t.Context(), PutRequest{
					WorkspaceID: tc.workspaceID, SessionID: tc.sessionID, Name: "notes.txt", Data: payload,
				})
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
		store, err := OpenFilesystemAttachmentStore(t.Context(), FilesystemStoreConfig{
			Root: root, Retention: testAttachmentRetention(),
			Limits: StoreLimits{MaxFileBytes: 4, AllowedMIME: DefaultAllowedMIMEs()},
		})
		if err != nil {
			t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
		}
		_, err = store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "notes.txt", Data: []byte("hello"),
		})
		if !errors.Is(err, ErrTooLarge) {
			t.Fatalf("Put() error = %v, want too large", err)
		}
	})

	t.Run("Should reject a payload larger than the total retention budget", func(t *testing.T) {
		t.Parallel()

		retention := testAttachmentRetention()
		retention.MaxBytes = 4
		store, err := OpenFilesystemAttachmentStore(t.Context(), FilesystemStoreConfig{
			Root: filepath.Join(t.TempDir(), "session-attachments"), Retention: retention,
			Limits: StoreLimits{MaxFileBytes: 8, AllowedMIME: DefaultAllowedMIMEs()},
		})
		if err != nil {
			t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
		}
		_, err = store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "notes.txt", Data: []byte("hello"),
		})
		if !errors.Is(err, ErrTooLarge) {
			t.Fatalf("Put() error = %v, want too large", err)
		}
	})

	t.Run("Should reject MIME outside the configured allowlist", func(t *testing.T) {
		t.Parallel()

		store, err := OpenFilesystemAttachmentStore(t.Context(), FilesystemStoreConfig{
			Root: filepath.Join(t.TempDir(), "session-attachments"), Retention: testAttachmentRetention(),
			Limits: StoreLimits{MaxFileBytes: 1024, AllowedMIME: []string{MIMETextPlain}},
		})
		if err != nil {
			t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
		}
		_, err = store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "notes.md", Data: []byte("# notes"),
		})
		if !errors.Is(err, ErrUnsupportedMIME) {
			t.Fatalf("Put() error = %v, want unsupported MIME", err)
		}
	})
}

func TestFilesystemAttachmentStoreRedactsPersistedFilename(t *testing.T) {
	t.Parallel()

	t.Run("Should redact sensitive values from persisted filenames", func(t *testing.T) {
		t.Parallel()

		store := openTestAttachmentStore(
			t,
			filepath.Join(t.TempDir(), "session-attachments"),
			testAttachmentRetention(),
		)
		const leakedName = "token=attachment-name-secret compozy_claim_attachment_123.txt"

		stored, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: leakedName, Data: []byte("plain notes"),
		})
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		if strings.Contains(stored.Name, "attachment-name-secret") ||
			strings.Contains(stored.Name, "compozy_claim_attachment_123") {
			t.Fatalf("Put() Name = %q, leaked filename credentials", stored.Name)
		}
		reopened, err := store.Stat(t.Context(), "ws_a", "sess_1", stored.ID)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if reopened.Name != stored.Name {
			t.Fatalf("Stat() Name = %q, want persisted redacted name %q", reopened.Name, stored.Name)
		}
	})
}

func TestOpenFilesystemAttachmentStoreValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		root        string
		retention   AttachmentRetention
		limits      StoreLimits
		wantMessage string
	}{
		{
			name:        "Should reject an empty root",
			retention:   testAttachmentRetention(),
			limits:      testAttachmentLimits(),
			wantMessage: "attachment root is required",
		},
		{
			name: "Should reject invalid retention",
			root: filepath.Join(t.TempDir(), "invalid-retention"),
			retention: AttachmentRetention{
				MaxCount: 0,
				MaxBytes: 1024,
				MaxAge:   time.Hour,
			},
			limits:      testAttachmentLimits(),
			wantMessage: "attachment max count must be greater than zero: 0",
		},
		{
			name:        "Should reject a non-positive file limit",
			root:        filepath.Join(t.TempDir(), "invalid-file-limit"),
			retention:   testAttachmentRetention(),
			limits:      StoreLimits{AllowedMIME: DefaultAllowedMIMEs()},
			wantMessage: "attachment max_file_bytes must be greater than zero: 0",
		},
		{
			name:        "Should reject an empty MIME allowlist",
			root:        filepath.Join(t.TempDir(), "empty-allowlist"),
			retention:   testAttachmentRetention(),
			limits:      StoreLimits{MaxFileBytes: 1024},
			wantMessage: "attachment allowed_mime must not be empty",
		},
		{
			name:      "Should reject a MIME outside the v1 allowlist",
			root:      filepath.Join(t.TempDir(), "unsupported-allowlist"),
			retention: testAttachmentRetention(),
			limits: StoreLimits{
				MaxFileBytes: 1024,
				AllowedMIME:  []string{"image/gif"},
			},
			wantMessage: "attachments: unsupported mime type \"image/gif\"",
		},
		{
			name:      "Should reject duplicate MIME entries after normalization",
			root:      filepath.Join(t.TempDir(), "duplicate-allowlist"),
			retention: testAttachmentRetention(),
			limits: StoreLimits{
				MaxFileBytes: 1024,
				AllowedMIME:  []string{MIMETextPlain, " TEXT/PLAIN "},
			},
			wantMessage: "attachment allowed_mime duplicates \"text/plain\"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := OpenFilesystemAttachmentStore(t.Context(), FilesystemStoreConfig{
				Root: tc.root, Retention: tc.retention, Limits: tc.limits,
			}); err == nil || !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf(
					"OpenFilesystemAttachmentStore() error = %v, want validation detail %q",
					err,
					tc.wantMessage,
				)
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
		store := openTestAttachmentStore(t, filepath.Join(t.TempDir(), "session-attachments"), retention)
		store.now = func() time.Time { return now }
		first, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "a.txt", Data: []byte("first"),
		})
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}
		now = now.Add(time.Second)
		second, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "b.txt", Data: []byte("second"),
		})
		if err != nil {
			t.Fatalf("Put(second) error = %v", err)
		}
		now = now.Add(time.Second)
		third, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "c.txt", Data: []byte("third"),
		})
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
		store := openTestAttachmentStore(t, filepath.Join(t.TempDir(), "session-attachments"), retention)
		store.now = func() time.Time { return now }
		old, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "old.txt", Data: []byte("12345"),
		})
		if err != nil {
			t.Fatalf("Put(old) error = %v", err)
		}
		now = now.Add(time.Second)
		current, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_b", SessionID: "sess_2", Name: "new.txt", Data: []byte("67890"),
		})
		if err != nil {
			t.Fatalf("Put(current) error = %v", err)
		}
		assertAttachmentMissing(t, store, "ws_a", "sess_1", old.ID)
		assertAttachmentPresent(t, store, "ws_b", "sess_2", current.ID)

		now = now.Add(time.Hour + time.Second)
		assertAttachmentMissing(t, store, "ws_b", "sess_2", current.ID)
	})

	t.Run("Should replace an expired attachment when the same bytes are put again", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 14, 6, 0, 0, 0, time.UTC)
		retention := testAttachmentRetention()
		retention.MaxAge = time.Hour
		store := openTestAttachmentStore(t, filepath.Join(t.TempDir(), "session-attachments"), retention)
		store.now = func() time.Time { return now }
		payload := []byte("same retained content")
		first, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "notes.txt", Data: payload,
		})
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}

		now = now.Add(retention.MaxAge + time.Second)
		second, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "notes.txt", Data: payload,
		})
		if err != nil {
			t.Fatalf("Put(second) error = %v", err)
		}
		if second.ID != first.ID {
			t.Fatalf("Put(second).ID = %q, want content-addressed ID %q", second.ID, first.ID)
		}
		if !second.CreatedAt.Equal(now) {
			t.Fatalf("Put(second).CreatedAt = %s, want %s", second.CreatedAt, now)
		}
	})

	t.Run("Should preserve pending-input pins until the queue releases them", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 14, 8, 0, 0, 0, time.UTC)
		retention := testAttachmentRetention()
		retention.MaxAge = time.Hour
		pins := &testRetentionPinSource{bySession: map[string]map[string]struct{}{}}
		store := openTestAttachmentStoreWithPins(t, filepath.Join(t.TempDir(), "session-attachments"), retention, pins)
		store.now = func() time.Time { return now }
		ref, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_queued", Name: "queued.txt", Data: []byte("queued"),
		})
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		pins.bySession["sess_queued"] = map[string]struct{}{ref.ID: {}}
		now = now.Add(2 * time.Hour)
		if err := store.Sweep(t.Context()); err != nil {
			t.Fatalf("Sweep(pinned) error = %v", err)
		}
		assertAttachmentPresent(t, store, "ws_a", "sess_queued", ref.ID)

		pins.bySession["sess_queued"] = map[string]struct{}{}
		if err := store.Sweep(t.Context()); err != nil {
			t.Fatalf("Sweep(released) error = %v", err)
		}
		assertAttachmentMissing(t, store, "ws_a", "sess_queued", ref.ID)
	})
}

func TestFilesystemAttachmentStoreFailureCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should remove published files when the writer reports failure", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join(t.TempDir(), "session-attachments")
		store := openTestAttachmentStore(t, root, testAttachmentRetention())
		store.writeFile = func(path string, content []byte, perm os.FileMode) error {
			if writeErr := os.WriteFile(path, content, perm); writeErr != nil {
				return writeErr
			}
			return errors.New("injected sync failure")
		}
		_, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "notes.txt", Data: []byte("payload"),
		})
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

	t.Run("Should recover incomplete attachment pairs when reopening", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join(t.TempDir(), "session-attachments")
		sessionPath := filepath.Join(root, "ws_a", "sess_1")
		if err := os.MkdirAll(sessionPath, 0o700); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		content := []byte("interrupted publish content")
		contentPath := filepath.Join(sessionPath, attachmentID(content))
		if err := os.WriteFile(contentPath, content, attachmentFileMode); err != nil {
			t.Fatalf("WriteFile(content) error = %v", err)
		}
		orphanMetaPath := metaPath(filepath.Join(sessionPath, attachmentID([]byte("interrupted deletion metadata"))))
		if err := os.WriteFile(orphanMetaPath, []byte("{}"), attachmentFileMode); err != nil {
			t.Fatalf("WriteFile(sidecar) error = %v", err)
		}

		store, err := OpenFilesystemAttachmentStore(t.Context(), FilesystemStoreConfig{
			Root: root, Retention: testAttachmentRetention(), Limits: testAttachmentLimits(),
		})
		if err != nil {
			t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
		}
		for _, path := range []string{contentPath, metaPath(contentPath), orphanMetaPath} {
			if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("Lstat(%q) error = %v, want missing recovered artifact", path, statErr)
			}
		}
		if _, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "notes.txt", Data: content,
		}); err != nil {
			t.Fatalf("Put() error = %v, want usable recovered store", err)
		}
	})
}

func TestFilesystemAttachmentStoreMetadataIntegrity(t *testing.T) {
	t.Parallel()

	t.Run("Should leave full content verification to requested-object reads", func(t *testing.T) {
		t.Parallel()

		store := openTestAttachmentStore(
			t,
			filepath.Join(t.TempDir(), "session-attachments"),
			testAttachmentRetention(),
		)
		ref, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "notes.txt", Data: []byte("payload"),
		})
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		contentPath := store.contentPath("ws_a", "sess_1", ref.ID)
		if err := os.WriteFile(contentPath, []byte("tamper!"), attachmentFileMode); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if err := store.Sweep(t.Context()); err != nil {
			t.Fatalf("Sweep() error = %v, want sidecar-only retention accounting", err)
		}
		if _, err := store.Stat(t.Context(), "ws_a", "sess_1", ref.ID); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("Stat() error = %v, want corrupt", err)
		}
	})

	t.Run("Should reject a sidecar whose byte count differs from content", func(t *testing.T) {
		t.Parallel()

		store := openTestAttachmentStore(
			t,
			filepath.Join(t.TempDir(), "session-attachments"),
			testAttachmentRetention(),
		)
		ref, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "notes.txt", Data: []byte("payload"),
		})
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

		store := openTestAttachmentStore(
			t,
			filepath.Join(t.TempDir(), "session-attachments"),
			testAttachmentRetention(),
		)
		ref, err := store.Put(t.Context(), PutRequest{
			WorkspaceID: "ws_a", SessionID: "sess_1", Name: "notes.txt", Data: []byte("payload"),
		})
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
	return openTestAttachmentStoreWithPins(t, root, retention, nil)
}

func openTestAttachmentStoreWithPins(
	t *testing.T,
	root string,
	retention AttachmentRetention,
	retentionPins RetentionPinSource,
) *FilesystemAttachmentStore {
	t.Helper()
	store, err := OpenFilesystemAttachmentStore(t.Context(), FilesystemStoreConfig{
		Root: root, Retention: retention, Limits: testAttachmentLimits(), RetentionPins: retentionPins,
	})
	if err != nil {
		t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
	}
	return store
}

func testAttachmentRetention() AttachmentRetention {
	return AttachmentRetention{MaxCount: 20, MaxBytes: 1 << 20, MaxAge: 24 * time.Hour}
}

type testRetentionPinSource struct {
	bySession map[string]map[string]struct{}
}

func (s *testRetentionPinSource) PinnedAttachmentIDs(
	_ context.Context,
	sessionID string,
) (map[string]struct{}, error) {
	return s.bySession[sessionID], nil
}

func testAttachmentLimits() StoreLimits {
	return StoreLimits{MaxFileBytes: 1 << 20, AllowedMIME: DefaultAllowedMIMEs()}
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
