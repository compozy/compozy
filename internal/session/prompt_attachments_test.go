package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

type promptAttachmentOpenerStub struct {
	data  map[string][]byte
	refs  map[string]attachmentspkg.AttachmentRef
	calls int
}

func (o *promptAttachmentOpenerStub) Open(
	_ context.Context,
	_ string,
	_ string,
	id string,
) (io.ReadCloser, attachmentspkg.AttachmentRef, error) {
	o.calls++
	data, ok := o.data[id]
	if !ok {
		return nil, attachmentspkg.AttachmentRef{}, attachmentspkg.ErrNotFound
	}
	ref, ok := o.refs[id]
	if !ok {
		digest := sha256.Sum256(data)
		ref = attachmentspkg.AttachmentRef{
			ID: id, Name: id + ".txt", MIMEType: "text/plain", Bytes: int64(len(data)),
			SHA256: fmt.Sprintf("%x", digest), Kind: attachmentspkg.KindFile,
		}
	}
	return io.NopCloser(bytes.NewReader(data)), ref, nil
}

var _ AttachmentOpener = (*promptAttachmentOpenerStub)(nil)

func TestResolvePromptAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve bytes that match admitted metadata", func(t *testing.T) {
		t.Parallel()

		data := []byte("document contents")
		opener := &promptAttachmentOpenerStub{data: map[string][]byte{"att-document": data}}
		manager := &Manager{attachmentOpener: opener}
		resolved, err := manager.resolvePromptAttachments(
			context.Background(),
			"ws-test",
			"sess-test",
			[]AttachmentMeta{promptAttachmentMeta("att-document", "notes.txt", "text/plain", data)},
			acp.Caps{},
		)
		if err != nil {
			t.Fatalf("resolvePromptAttachments() error = %v", err)
		}
		if len(resolved) != 1 || !bytes.Equal(resolved[0].Data, data) {
			t.Fatalf("resolved attachments = %#v, want original bytes", resolved)
		}
	})

	t.Run("Should reject a digest mismatch without exposing attachment bytes", func(t *testing.T) {
		t.Parallel()

		secret := []byte("private attachment contents")
		opener := &promptAttachmentOpenerStub{
			data: map[string][]byte{"att-secret": secret},
			refs: map[string]attachmentspkg.AttachmentRef{
				"att-secret": {
					ID: "att-secret", Name: "secret.txt", MIMEType: "text/plain",
					SHA256: strings.Repeat("0", 64),
				},
			},
		}
		manager := &Manager{attachmentOpener: opener}
		meta := promptAttachmentMeta("att-secret", "secret.txt", "text/plain", []byte("different"))
		_, err := manager.resolvePromptAttachments(
			context.Background(), "ws-test", "sess-test", []AttachmentMeta{meta}, acp.Caps{},
		)
		if !errors.Is(err, ErrPromptAttachmentDigestMismatch) {
			t.Fatalf("resolvePromptAttachments() error = %v, want %v", err, ErrPromptAttachmentDigestMismatch)
		}
		if strings.Contains(err.Error(), string(secret)) {
			t.Fatalf("resolvePromptAttachments() error exposed attachment bytes: %v", err)
		}
	})

	t.Run("Should reject unsupported image input before opening bytes", func(t *testing.T) {
		t.Parallel()

		data := []byte("image contents")
		opener := &promptAttachmentOpenerStub{
			data: map[string][]byte{"att-image": data},
			refs: map[string]attachmentspkg.AttachmentRef{
				"att-image": storedPromptAttachmentRef("att-image", "photo.png", "image/png", data),
			},
		}
		manager := &Manager{attachmentOpener: opener}
		_, err := manager.resolvePromptAttachments(
			context.Background(),
			"ws-test",
			"sess-test",
			[]AttachmentMeta{promptAttachmentMeta("att-image", "photo.png", "image/png", data)},
			acp.Caps{},
		)
		if !errors.Is(err, ErrPromptImagesUnsupported) {
			t.Fatalf("resolvePromptAttachments() error = %v, want %v", err, ErrPromptImagesUnsupported)
		}
		if opener.calls != 0 {
			t.Fatalf("attachment opener calls = %d, want 0 before capability gate", opener.calls)
		}
	})

	t.Run("Should reject unsupported PDF input before opening bytes", func(t *testing.T) {
		t.Parallel()

		data := []byte("pdf contents")
		opener := &promptAttachmentOpenerStub{
			data: map[string][]byte{"att-pdf": data},
			refs: map[string]attachmentspkg.AttachmentRef{
				"att-pdf": storedPromptAttachmentRef("att-pdf", "report.pdf", "application/pdf", data),
			},
		}
		manager := &Manager{attachmentOpener: opener}
		_, err := manager.resolvePromptAttachments(
			context.Background(),
			"ws-test",
			"sess-test",
			[]AttachmentMeta{promptAttachmentMeta("att-pdf", "report.pdf", "application/pdf", data)},
			acp.Caps{},
		)
		if !errors.Is(err, ErrPromptFilesUnsupported) {
			t.Fatalf("resolvePromptAttachments() error = %v, want %v", err, ErrPromptFilesUnsupported)
		}
		if opener.calls != 0 {
			t.Fatalf("attachment opener calls = %d, want 0 before capability gate", opener.calls)
		}
	})
}

func storedPromptAttachmentRef(id string, name string, mimeType string, data []byte) attachmentspkg.AttachmentRef {
	digest := sha256.Sum256(data)
	kind := attachmentspkg.KindFile
	if strings.HasPrefix(mimeType, "image/") {
		kind = attachmentspkg.KindImage
	}
	return attachmentspkg.AttachmentRef{
		ID: id, Name: name, MIMEType: mimeType, Bytes: int64(len(data)),
		SHA256: fmt.Sprintf("%x", digest), Kind: kind,
	}
}

func TestSendPromptDispatchesAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should dispatch an attachment-only prompt with resolved bytes", func(t *testing.T) {
		t.Parallel()

		attachmentID := "att_" + strings.Repeat("a", 64)
		data := []byte("plain attachment")
		opener := &promptAttachmentOpenerStub{
			data: map[string][]byte{attachmentID: data},
			refs: map[string]attachmentspkg.AttachmentRef{
				attachmentID: storedPromptAttachmentRef(attachmentID, "notes.txt", "text/plain", data),
			},
		}
		admissionStore := openManagerInputQueueStore(t)
		h := newHarness(
			t,
			WithAttachmentOpener(opener),
			WithSessionPromptAdmissionStore(admissionStore),
		)
		registerManagerInputQueueWorkspace(t, admissionStore, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, admissionStore, h, sess)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })

		opts := SendPromptOpts{
			MessageID: "message-attachment-only", IdempotencyKey: "idempotency-attachment-only",
			Attachments: []AttachmentMeta{
				promptAttachmentMeta(attachmentID, "spoofed.md", "text/markdown", data),
			},
		}
		result, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, opts)
		if err != nil {
			t.Fatalf("SendPrompt() error = %v", err)
		}
		collectEvents(t, result.Events)
		replayed, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, opts)
		if err != nil {
			t.Fatalf("SendPrompt(replay) error = %v", err)
		}
		if !replayed.Replayed {
			t.Fatalf("SendPrompt(replay).Replayed = false, want true")
		}
		calls := managerPromptCalls(h)
		if len(calls) != 1 || len(calls[0].Attachments) != 1 {
			t.Fatalf("driver prompt calls = %#v, want one attachment", calls)
		}
		if calls[0].Message != "" || calls[0].Attachments[0].Name != "notes.txt" ||
			calls[0].Attachments[0].MIMEType != "text/plain" ||
			!bytes.Equal(calls[0].Attachments[0].Data, data) {
			t.Fatalf("driver prompt = %#v, want empty message with resolved attachment", calls[0])
		}
		assertPersistedPromptAttachment(t, sess, attachmentID, "notes.txt", "text/plain")
	})

	t.Run("Should reject unsupported image input before calling the driver", func(t *testing.T) {
		t.Parallel()

		data := []byte("image attachment")
		opener := &promptAttachmentOpenerStub{
			data: map[string][]byte{"att-image": data},
			refs: map[string]attachmentspkg.AttachmentRef{
				"att-image": storedPromptAttachmentRef("att-image", "photo.png", "image/png", data),
			},
		}
		h := newHarness(t, WithAttachmentOpener(opener))
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })

		_, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message:     "inspect this",
			Attachments: []AttachmentMeta{promptAttachmentMeta("att-image", "spoofed.txt", "text/plain", data)},
		})
		if !errors.Is(err, ErrPromptImagesUnsupported) {
			t.Fatalf("SendPrompt() error = %v, want %v", err, ErrPromptImagesUnsupported)
		}
		if got := len(managerPromptCalls(h)); got != 0 {
			t.Fatalf("driver prompt calls = %d, want 0 after capability rejection", got)
		}
	})
}

func assertPersistedPromptAttachment(
	t *testing.T,
	session *Session,
	id string,
	name string,
	mimeType string,
) {
	t.Helper()
	for _, storedEvent := range readStoredEvents(t, session) {
		decoded, err := transcript.UnmarshalAgentEvent(storedEvent.Content)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		attachments := decoded.Attachments()
		if len(attachments) == 0 {
			continue
		}
		if len(attachments) != 1 || attachments[0].ID != id || attachments[0].Name != name ||
			attachments[0].MIMEType != mimeType {
			t.Fatalf("persisted attachments = %#v, want authoritative stored metadata", attachments)
		}
		return
	}
	t.Fatal("persisted prompt attachment not found")
}

func promptAttachmentMeta(id string, name string, mimeType string, data []byte) AttachmentMeta {
	digest := sha256.Sum256(data)
	kind := AttachmentKindFile
	if strings.HasPrefix(mimeType, "image/") {
		kind = AttachmentKindImage
	}
	return AttachmentMeta{
		ID:       id,
		Name:     name,
		MIMEType: mimeType,
		Bytes:    int64(len(data)),
		SHA256:   fmt.Sprintf("%x", digest),
		Kind:     kind,
	}
}
