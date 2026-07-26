package daemon

import (
	"context"
	"testing"
)

func TestDaemonDrainState(t *testing.T) {
	t.Parallel()

	d := &Daemon{}
	if got := d.DrainState(); got != DrainStateActive {
		t.Fatalf("DrainState(initial) = %q, want %q", got, DrainStateActive)
	}
	if err := d.Drain(context.Background()); err != nil {
		t.Fatalf("Drain(first) error = %v", err)
	}
	if err := d.Drain(context.Background()); err != nil {
		t.Fatalf("Drain(second) error = %v", err)
	}
	if got := d.DrainState(); got != DrainStateDraining {
		t.Fatalf("DrainState(draining) = %q, want %q", got, DrainStateDraining)
	}
	if err := d.Undrain(context.Background()); err != nil {
		t.Fatalf("Undrain(first) error = %v", err)
	}
	if err := d.Undrain(context.Background()); err != nil {
		t.Fatalf("Undrain(second) error = %v", err)
	}
	if got := d.DrainState(); got != DrainStateActive {
		t.Fatalf("DrainState(active) = %q, want %q", got, DrainStateActive)
	}
}

func TestDaemonBeginBootShouldRestoreNewWorkAdmission(t *testing.T) {
	t.Parallel()

	d := &Daemon{}
	if err := d.Drain(context.Background()); err != nil {
		t.Fatalf("Drain() error = %v", err)
	}
	if err := d.beginBoot(); err != nil {
		t.Fatalf("beginBoot() error = %v", err)
	}
	if got := d.DrainState(); got != DrainStateActive {
		t.Fatalf("DrainState(begin boot) = %q, want %q", got, DrainStateActive)
	}
}
