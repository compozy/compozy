package model

import "testing"

func TestRuntimeConfigApplyDefaultsKeepsUnlimitedTailHistory(t *testing.T) {
	t.Parallel()

	cfg := &RuntimeConfig{TailLines: 0}
	cfg.ApplyDefaults()

	if cfg.TailLines != 0 {
		t.Fatalf("expected tail-lines default to remain unlimited (0), got %d", cfg.TailLines)
	}
}
