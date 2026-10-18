package testutil

import (
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// ConfigForTest returns the default isolated API test configuration.
func ConfigForTest(homePaths compozyconfig.HomePaths) compozyconfig.Config {
	return compozyconfig.DefaultWithHome(homePaths)
}

// NewHomeConfig creates an isolated API test home and its configuration.
func NewHomeConfig(t *testing.T) (compozyconfig.HomePaths, compozyconfig.Config) {
	t.Helper()
	homePaths := NewTestHomePaths(t)
	return homePaths, ConfigForTest(homePaths)
}
