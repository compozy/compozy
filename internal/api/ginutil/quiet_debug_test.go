package ginutil

import (
	"bytes"
	"net/http"
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
}
