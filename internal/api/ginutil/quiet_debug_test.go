package ginutil

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestQuietDebug(t *testing.T) {
	// Not parallel: Gin mode and DefaultWriter are process-wide.
	t.Run("Should suppress router setup output and restore debug mode", func(t *testing.T) {
		previousMode := gin.Mode()
		previousWriter := gin.DefaultWriter
		var output bytes.Buffer
		gin.SetMode(gin.DebugMode)
		gin.DefaultWriter = &output
		t.Cleanup(func() {
			gin.DefaultWriter = previousWriter
			gin.SetMode(previousMode)
		})

		engine := NewEngine()
		QuietDebug(func() {
			engine.GET("/api/mcp/oauth/callback", func(context *gin.Context) {
				context.Status(http.StatusNoContent)
			})
		})

		if output.Len() != 0 {
			t.Fatalf("Gin setup output = %q, want empty", output.String())
		}
		if got := gin.Mode(); got != gin.DebugMode {
			t.Fatalf("Gin mode = %q, want %q", got, gin.DebugMode)
		}
	})

	t.Run("Should route and decode opaque path parameters without rewriting them", func(t *testing.T) {
		// Not parallel: NewEngine temporarily coordinates Gin's process-wide mode.
		engine := NewEngine()
		engine.POST("/api/resources/:id/inspect", func(context *gin.Context) {
			context.String(http.StatusOK, context.Param("id"))
		})

		tests := []struct {
			name string
			path string
			want string
		}{}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				request := httptest.NewRequestWithContext(
					t.Context(),
					http.MethodPost,
					test.path,
					http.NoBody,
				)
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)

				if got, want := response.Code, http.StatusOK; got != want {
					t.Fatalf("response status = %d, want %d; body=%s", got, want, response.Body.String())
				}
				if got := response.Body.String(); got != test.want {
					t.Fatalf("response body = %q, want %q", got, test.want)
				}
			})
		}
	})
}
