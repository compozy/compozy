package ginmode

import (
	"sync"

	"github.com/gin-gonic/gin"
)

var once sync.Once

// EnsureGinTestMode switches Gin into TestMode exactly once per process.
func EnsureGinTestMode() {
	once.Do(func() {
		gin.SetMode(gin.TestMode)
	})
}
