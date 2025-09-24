package helpers

import (
	"sync"

	"github.com/gin-gonic/gin"
)

var ginModeOnce sync.Once

func EnsureGinTestMode() {
	ginModeOnce.Do(func() {
		gin.SetMode(gin.TestMode)
	})
}
