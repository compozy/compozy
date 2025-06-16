package utils

import (
	"sync"
	"testing"

	"github.com/compozy/compozy/pkg/logger"
)

var loggerOnce sync.Once

func InitLogger(t *testing.T) {
	loggerOnce.Do(func() {
		if err := logger.InitForTests(); err != nil {
			// Log the error but don't fail test initialization
			t.Errorf("Warning: failed to initialize logger for tests: %v\n", err)
		}
	})
}
