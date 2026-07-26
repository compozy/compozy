package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
)

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
