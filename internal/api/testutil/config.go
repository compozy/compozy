package testutil

import (
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// ConfigWithDisabledNetwork returns a default test config with networking turned off.
func ConfigWithDisabledNetwork(homePaths compozyconfig.HomePaths) compozyconfig.Config {
	cfg := compozyconfig.DefaultWithHome(homePaths)
	cfg.Network.Enabled = false
	return cfg
}

// NewDisabledNetworkHomeConfig creates one test home and derives a disabled-network config from it.
func NewDisabledNetworkHomeConfig(t *testing.T) (compozyconfig.HomePaths, compozyconfig.Config) {
	t.Helper()

	homePaths := NewTestHomePaths(t)
	return homePaths, ConfigWithDisabledNetwork(homePaths)
}
