package size

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// BodySizeLimiter limits the request body size for the route group.
func BodySizeLimiter(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit <= 0 {
			c.Next()
			return
		}
		if c.Request.ContentLength > 0 && c.Request.ContentLength > limit {
			c.Writer.Header().Set("Connection", "close")
			if err := c.Request.Body.Close(); err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":   "failed to close request body",
					"details": fmt.Sprintf("close error: %v", err),
				})
				return
			}
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "request body too large",
				"details": "max bytes allowed: " + fmt.Sprint(limit),
			})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
