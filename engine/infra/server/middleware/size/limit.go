package size

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BodySizeLimiter limits the request body size for the route group.
func BodySizeLimiter(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
