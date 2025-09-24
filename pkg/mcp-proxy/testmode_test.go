package mcpproxy

import (
	"sync"

	"github.com/gin-gonic/gin"
)

var ginModeOnce sync.Once

func ensureGinTestMode() {
	ginModeOnce.Do(func() {
		gin.SetMode(gin.TestMode)
	})
}
