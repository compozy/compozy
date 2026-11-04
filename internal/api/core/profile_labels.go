package core

import (
	"context"

	"github.com/compozy/compozy/internal/profiling"

	"github.com/gin-gonic/gin"
)

func ProfileLabels(c *gin.Context) {
	profiling.Do(c.Request.Context(), func(ctx context.Context) {
		if ctx != c.Request.Context() {
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}, "route", c.FullPath())
}
