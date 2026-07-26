// Package ginutil owns process-wide Gin runtime coordination shared by AGH transports.
package ginutil

import (
	"sync"

	"github.com/gin-gonic/gin"
)

var debugModeMu sync.Mutex

// NewEngine constructs a Gin engine without emitting debug-mode setup output.
func NewEngine() *gin.Engine {
	var engine *gin.Engine
	QuietDebug(func() { engine = gin.New() })
	return engine
}

// QuietDebug suppresses Gin's debug-mode setup output while fn constructs or registers a router.
func QuietDebug(fn func()) {
	debugModeMu.Lock()
	defer debugModeMu.Unlock()

	previousMode := gin.Mode()
	if previousMode == gin.DebugMode {
		gin.SetMode(gin.ReleaseMode)
		defer gin.SetMode(previousMode)
	}
	fn()
}
