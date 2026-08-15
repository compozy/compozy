package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/gin-gonic/gin"
)

func TestSessionAttachmentRequestBodyLimitUsesConfiguredFileCeiling(t *testing.T) {
	t.Parallel()

	const maxFileBytes int64 = 256
	wantBodyLimit := maxFileBytes + core.SessionAttachmentMultipartOverheadBytes
	tests := []struct {
		name       string
		bodyBytes  int64
		wantStatus int
		wantError  string
	}{
		{
			name:       "Should accept the configured file ceiling plus multipart overhead",
			bodyBytes:  wantBodyLimit,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "Should reject a body above the configured file ceiling plus multipart overhead",
			bodyBytes:  wantBodyLimit + 1,
			wantStatus: http.StatusRequestEntityTooLarge,
			wantError:  errRequestBodyTooLarge.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newDefaultServer(newTestHomePaths(t))
			server.config.Session.Attachments.MaxFileBytes = maxFileBytes
			server.host = "127.0.0.1"
			server.ensureEngine()
			server.engine.POST(
				"/api/workspaces/:workspace_id/sessions/:session_id/attachments",
				func(c *gin.Context) { c.Status(http.StatusNoContent) },
			)

			req := httptest.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				"http://127.0.0.1/api/workspaces/ws-workspace/sessions/sess-123/attachments",
				strings.NewReader(strings.Repeat("x", int(tt.bodyBytes))),
			)
			req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
			recorder := httptest.NewRecorder()

			server.engine.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if tt.wantError != "" {
				var payload contract.ErrorPayload
				decodeJSONResponse(t, recorder, &payload)
				if payload.Error != tt.wantError {
					t.Fatalf("error = %q, want %q", payload.Error, tt.wantError)
				}
			}
		})
	}
}

func TestRequestBodyLimitRejectsOversizedChunkedAPIRequestsContract(t *testing.T) {
	t.Parallel()

	t.Run("Should return 413 when MaxBytesReader rejects an oversized chunked JSON body", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))
		body := io.MultiReader(
			strings.NewReader(`{"message":"`),
			strings.NewReader(strings.Repeat("x", int(maxAPIRequestBodyBytes)+1)),
			strings.NewReader(`"}`),
		)
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://127.0.0.1/api/workspaces/ws-workspace/sessions/sess-123/prompt",
			body,
		)
		req.ContentLength = -1
		req.TransferEncoding = []string{"chunked"}
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		engine.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				recorder.Code,
				http.StatusRequestEntityTooLarge,
				recorder.Body.String(),
			)
		}

		var payload contract.ErrorPayload
		decodeJSONResponse(t, recorder, &payload)
		if payload.Error != errRequestBodyTooLarge.Error() {
			t.Fatalf("error = %q, want %q", payload.Error, errRequestBodyTooLarge.Error())
		}
	})
}
