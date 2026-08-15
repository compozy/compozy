package core_test

// Suite: session attachment core handlers
// Invariant: attachment operations require the session to belong to the resolved workspace before storage access.
// Boundary IN: multipart upload, scoped attachment byte reads, and scoped deletion requests.
// Boundary OUT: HTTP/UDS route registration, transport body limits, and CLI behavior.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	"github.com/compozy/compozy/internal/session"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

type sessionAttachmentFixture struct {
	Engine       *gin.Engine
	Store        *attachmentspkg.FilesystemAttachmentStore
	MaxFileBytes int64
}

func TestSessionAttachmentHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should upload and read attachment bytes with stored metadata", func(t *testing.T) {
		t.Parallel()
		fixture := newSessionAttachmentFixture(t)
		data := []byte("hello")

		upload := serveSessionAttachmentRequest(t, fixture.Engine, newMultipartAttachmentRequest(
			t,
			http.MethodPost,
			"/api/workspaces/workspace/sessions/sess-1/attachments",
			"notes.txt",
			data,
		))
		if upload.Code != http.StatusCreated {
			t.Fatalf("upload status = %d, want %d; body=%s", upload.Code, http.StatusCreated, upload.Body.String())
		}
		var uploadPayload contract.SessionAttachmentUploadResponse
		testutil.DecodeJSONResponse(t, upload, &uploadPayload)
		ref := uploadPayload.Attachment
		if ref.ID == "" || ref.Name != "notes.txt" || ref.MIMEType != attachmentspkg.MIMETextPlain ||
			ref.Bytes != int64(len(data)) || ref.SHA256 == "" || ref.Kind != attachmentspkg.KindFile || ref.CreatedAt.IsZero() {
			t.Fatalf("upload attachment = %#v, want complete stored metadata", ref)
		}
		stored, storedRef, err := fixture.Store.Open(t.Context(), "stable-workspace", "sess-1", ref.ID)
		if err != nil {
			t.Fatalf("Store.Open() error = %v", err)
		}
		storedBytes, readErr := io.ReadAll(stored)
		closeErr := stored.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("stored attachment read errors = read:%v close:%v", readErr, closeErr)
		}
		if storedRef.ID != ref.ID || !bytes.Equal(storedBytes, data) {
			t.Fatalf("stored attachment = %#v %q, want id %q bytes %q", storedRef, storedBytes, ref.ID, data)
		}

		read := serveSessionAttachmentRequest(t, fixture.Engine, httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/api/workspaces/workspace/sessions/sess-1/attachments/"+ref.ID+"/bytes",
			http.NoBody,
		))
		if read.Code != http.StatusOK {
			t.Fatalf("read status = %d, want %d; body=%s", read.Code, http.StatusOK, read.Body.String())
		}
		if !bytes.Equal(read.Body.Bytes(), data) {
			t.Fatalf("read body = %q, want %q", read.Body.Bytes(), data)
		}
		if got := read.Header().Get("Content-Type"); got != attachmentspkg.MIMETextPlain {
			t.Fatalf("Content-Type = %q, want %q", got, attachmentspkg.MIMETextPlain)
		}
		if got := read.Header().Get("Content-Length"); got != "5" {
			t.Fatalf("Content-Length = %q, want 5", got)
		}
	})

	t.Run("Should reject an upload over the configured file ceiling", func(t *testing.T) {
		t.Parallel()
		fixture := newSessionAttachmentFixture(t)
		data := bytes.Repeat([]byte("a"), int(fixture.MaxFileBytes+1))

		response := serveSessionAttachmentRequest(t, fixture.Engine, newMultipartAttachmentRequest(
			t,
			http.MethodPost,
			"/api/workspaces/workspace/sessions/sess-1/attachments",
			"large.txt",
			data,
		))
		if response.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("oversized status = %d, want %d; body=%s", response.Code, http.StatusRequestEntityTooLarge, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "max_file_bytes") || !strings.Contains(response.Body.String(), "64") {
			t.Fatalf("oversized body = %q, want actionable configured limit", response.Body.String())
		}
	})

	t.Run("Should reject an unsupported sniffed MIME type", func(t *testing.T) {
		t.Parallel()
		fixture := newSessionAttachmentFixture(t)

		response := serveSessionAttachmentRequest(t, fixture.Engine, newMultipartAttachmentRequest(
			t,
			http.MethodPost,
			"/api/workspaces/workspace/sessions/sess-1/attachments",
			"payload.bin",
			[]byte{0, 1, 2},
		))
		if response.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("unsupported MIME status = %d, want %d; body=%s", response.Code, http.StatusUnsupportedMediaType, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "allowed types are") ||
			!strings.Contains(response.Body.String(), attachmentspkg.MIMETextPlain) {
			t.Fatalf("unsupported MIME body = %q, want the configured allowed types", response.Body.String())
		}
	})

	t.Run("Should reject multipart requests with an extra field", func(t *testing.T) {
		t.Parallel()
		fixture := newSessionAttachmentFixture(t)

		response := serveSessionAttachmentRequest(t, fixture.Engine, newMultipartAttachmentRequestWithExtraField(
			t,
			"/api/workspaces/workspace/sessions/sess-1/attachments",
		))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("extra field status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "exactly one attachment file field") {
			t.Fatalf("extra field body = %q, want single-file error", response.Body.String())
		}
	})

	t.Run("Should keep duplicate content idempotent within one session", func(t *testing.T) {
		t.Parallel()
		fixture := newSessionAttachmentFixture(t)
		data := []byte("same bytes")
		path := "/api/workspaces/workspace/sessions/sess-1/attachments"

		first := serveSessionAttachmentRequest(t, fixture.Engine, newMultipartAttachmentRequest(t, http.MethodPost, path, "same.txt", data))
		second := serveSessionAttachmentRequest(t, fixture.Engine, newMultipartAttachmentRequest(t, http.MethodPost, path, "same.txt", data))
		if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
			t.Fatalf("duplicate upload statuses = %d, %d, want %d; bodies=%s / %s", first.Code, second.Code, http.StatusCreated, first.Body.String(), second.Body.String())
		}
		var firstPayload, secondPayload contract.SessionAttachmentUploadResponse
		testutil.DecodeJSONResponse(t, first, &firstPayload)
		testutil.DecodeJSONResponse(t, second, &secondPayload)
		if firstPayload.Attachment.ID != secondPayload.Attachment.ID {
			t.Fatalf("duplicate IDs = %q, %q, want the same content identity", firstPayload.Attachment.ID, secondPayload.Attachment.ID)
		}
	})

	t.Run("Should reject a foreign workspace before touching storage", func(t *testing.T) {
		t.Parallel()
		fixture := newSessionAttachmentFixture(t)
		data := []byte("must stay private")
		response := serveSessionAttachmentRequest(t, fixture.Engine, newMultipartAttachmentRequest(
			t,
			http.MethodPost,
			"/api/workspaces/other-workspace/sessions/sess-1/attachments",
			"private.txt",
			data,
		))
		if response.Code != http.StatusNotFound {
			t.Fatalf("foreign workspace status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
		}

		digest := sha256.Sum256(data)
		id := "att_" + hex.EncodeToString(digest[:])
		if ref, err := fixture.Store.Stat(t.Context(), "stable-workspace", "sess-1", id); err == nil {
			t.Fatalf("foreign upload stable scope returned ref %#v", ref)
		} else if !errors.Is(err, attachmentspkg.ErrNotFound) {
			t.Fatalf("foreign upload stable scope stat error = %v, want %v", err, attachmentspkg.ErrNotFound)
		}
		if ref, err := fixture.Store.Stat(t.Context(), "stable-other-workspace", "sess-1", id); err == nil {
			t.Fatalf("foreign upload other scope returned ref %#v", ref)
		} else if !errors.Is(err, attachmentspkg.ErrNotFound) {
			t.Fatalf("foreign upload other scope stat error = %v, want %v", err, attachmentspkg.ErrNotFound)
		}
	})

	t.Run("Should delete an attachment and hide it afterward", func(t *testing.T) {
		t.Parallel()
		fixture := newSessionAttachmentFixture(t)
		path := "/api/workspaces/workspace/sessions/sess-1/attachments"
		upload := serveSessionAttachmentRequest(t, fixture.Engine, newMultipartAttachmentRequest(t, http.MethodPost, path, "delete.txt", []byte("delete me")))
		if upload.Code != http.StatusCreated {
			t.Fatalf("upload status = %d, want %d; body=%s", upload.Code, http.StatusCreated, upload.Body.String())
		}
		var payload contract.SessionAttachmentUploadResponse
		testutil.DecodeJSONResponse(t, upload, &payload)

		deleteResponse := serveSessionAttachmentRequest(t, fixture.Engine, httptest.NewRequestWithContext(
			t.Context(),
			http.MethodDelete,
			path+"/"+payload.Attachment.ID,
			http.NoBody,
		))
		if deleteResponse.Code != http.StatusNoContent || deleteResponse.Body.Len() != 0 {
			t.Fatalf("delete response = status %d body %q, want 204 with empty body", deleteResponse.Code, deleteResponse.Body.String())
		}

		readResponse := serveSessionAttachmentRequest(t, fixture.Engine, httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			path+"/"+payload.Attachment.ID+"/bytes",
			http.NoBody,
		))
		if readResponse.Code != http.StatusNotFound {
			t.Fatalf("read after delete status = %d, want %d; body=%s", readResponse.Code, http.StatusNotFound, readResponse.Body.String())
		}
	})
}

func newSessionAttachmentFixture(t *testing.T) sessionAttachmentFixture {
	t.Helper()
	const maxFileBytes int64 = 64
	homePaths := testutil.NewTestHomePaths(t)
	cfg := testutil.ConfigWithDisabledNetwork(homePaths)
	cfg.Session.Attachments.MaxFileBytes = maxFileBytes
	store, err := attachmentspkg.OpenFilesystemAttachmentStore(
		t.Context(),
		filepath.Join(t.TempDir(), "session-attachments"),
		attachmentspkg.AttachmentRetention{MaxCount: 16, MaxBytes: 1 << 20, MaxAge: time.Hour},
		attachmentspkg.StoreLimits{MaxFileBytes: maxFileBytes, AllowedMIME: cfg.Session.Attachments.AllowedMIME},
	)
	if err != nil {
		t.Fatalf("OpenFilesystemAttachmentStore() error = %v", err)
	}
	manager := testutil.StubSessionManager{StatusFn: func(ctx context.Context, id string) (*session.Info, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if id != "sess-1" {
			return nil, session.ErrSessionNotFound
		}
		info := testutil.NewSessionInfo(id)
		info.WorkspaceID = "registry-workspace"
		return info, nil
	}}
	workspaces := testutil.StubWorkspaceService{ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
		if err := ctx.Err(); err != nil {
			return workspacepkg.ResolvedWorkspace{}, err
		}
		switch strings.TrimSpace(ref) {
		case "workspace":
			return workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "registry-workspace", Name: "workspace"},
				WorkspaceID: "stable-workspace",
			}, nil
		case "other-workspace":
			return workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "registry-other-workspace", Name: "other-workspace"},
				WorkspaceID: "stable-other-workspace",
			}, nil
		default:
			return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
		}
	}}
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		Sessions:           manager,
		Workspaces:         workspaces,
		SessionAttachments: store,
		Config:             cfg,
		MaskInternalErrors: false,
	})
	engine := gin.New()
	workspaceSessions := engine.Group("/api/workspaces/:workspace_id/sessions/:session_id")
	workspaceSessions.POST("/attachments", handlers.UploadSessionAttachment)
	workspaceSessions.GET("/attachments/:attachment_id/bytes", handlers.ReadSessionAttachmentBytes)
	workspaceSessions.DELETE("/attachments/:attachment_id", handlers.DeleteSessionAttachment)
	return sessionAttachmentFixture{Engine: engine, Store: store, MaxFileBytes: maxFileBytes}
}

func newMultipartAttachmentRequest(t *testing.T, method string, path string, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if bytesWritten, err := part.Write(data); err != nil {
		t.Fatalf("multipart file write error = %v", err)
	} else if bytesWritten != len(data) {
		t.Fatalf("multipart file bytes written = %d, want %d", bytesWritten, len(data))
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart writer close error = %v", err)
	}
	request := httptest.NewRequestWithContext(t.Context(), method, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func newMultipartAttachmentRequestWithExtraField(t *testing.T, path string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "notes.txt")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte("hello")); err != nil {
		t.Fatalf("multipart file write error = %v", err)
	}
	if err := writer.WriteField("caption", "not allowed"); err != nil {
		t.Fatalf("multipart extra field write error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart writer close error = %v", err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func serveSessionAttachmentRequest(t *testing.T, engine http.Handler, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}
