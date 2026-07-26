package testutil

import (
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

// ConfigWithDisabledNetwork returns a default test config with networking turned off.
func ConfigWithDisabledNetwork(homePaths aghconfig.HomePaths) aghconfig.Config {
	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.Network.Enabled = false
	return cfg
}

// NewDisabledNetworkHomeConfig creates one test home and derives a disabled-network config from it.
func NewDisabledNetworkHomeConfig(t *testing.T) (aghconfig.HomePaths, aghconfig.Config) {
	t.Helper()

	homePaths := NewTestHomePaths(t)
	return homePaths, ConfigWithDisabledNetwork(homePaths)
}
