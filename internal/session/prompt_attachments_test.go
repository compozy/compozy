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
)

type promptAttachmentOpenerStub struct {
	data  map[string][]byte
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
	return io.NopCloser(bytes.NewReader(data)), attachmentspkg.AttachmentRef{ID: id}, nil
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
		opener := &promptAttachmentOpenerStub{data: map[string][]byte{"att-secret": secret}}
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
		opener := &promptAttachmentOpenerStub{data: map[string][]byte{"att-image": data}}
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
		opener := &promptAttachmentOpenerStub{data: map[string][]byte{"att-pdf": data}}
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

func TestSendPromptDispatchesAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should dispatch an attachment-only prompt with resolved bytes", func(t *testing.T) {
		t.Parallel()

		data := []byte("plain attachment")
		opener := &promptAttachmentOpenerStub{data: map[string][]byte{"att-text": data}}
		h := newHarness(t, WithAttachmentOpener(opener))
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })

		result, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Attachments: []AttachmentMeta{promptAttachmentMeta("att-text", "notes.txt", "text/plain", data)},
		})
		if err != nil {
			t.Fatalf("SendPrompt() error = %v", err)
		}
		collectEvents(t, result.Events)
		calls := managerPromptCalls(h)
		if len(calls) != 1 || len(calls[0].Attachments) != 1 {
			t.Fatalf("driver prompt calls = %#v, want one attachment", calls)
		}
		if calls[0].Message != "" || !bytes.Equal(calls[0].Attachments[0].Data, data) {
			t.Fatalf("driver prompt = %#v, want empty message with resolved attachment", calls[0])
		}
	})

	t.Run("Should reject unsupported image input before calling the driver", func(t *testing.T) {
		t.Parallel()

		data := []byte("image attachment")
		opener := &promptAttachmentOpenerStub{data: map[string][]byte{"att-image": data}}
		h := newHarness(t, WithAttachmentOpener(opener))
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })

		_, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message:     "inspect this",
			Attachments: []AttachmentMeta{promptAttachmentMeta("att-image", "photo.png", "image/png", data)},
		})
		if !errors.Is(err, ErrPromptImagesUnsupported) {
			t.Fatalf("SendPrompt() error = %v, want %v", err, ErrPromptImagesUnsupported)
		}
		if got := len(managerPromptCalls(h)); got != 0 {
			t.Fatalf("driver prompt calls = %d, want 0 after capability rejection", got)
		}
	})
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
