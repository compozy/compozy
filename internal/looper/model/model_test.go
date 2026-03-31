package model

import "testing"

func TestRuntimeConfigApplyDefaultsSetsSignalPort(t *testing.T) {
	t.Parallel()

	cfg := &RuntimeConfig{}
	cfg.ApplyDefaults()

	if cfg.SignalPort != DefaultSignalPort {
		t.Fatalf("SignalPort = %d, want %d", cfg.SignalPort, DefaultSignalPort)
	}
}
