package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/doctor"
)

// bootHostLoadMonitor starts host CPU pressure sampling and attaches it to the
// daemon admission gate so background work yields while the host is saturated.
func (d *Daemon) bootHostLoadMonitor(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	cfg := state.cfg.Daemon.Load
	monitor := admission.NewLoadMonitor(admission.LoadMonitorConfig{
		Threshold: cfg.AdmissionThreshold,
		MaxWait:   cfg.AdmissionMaxWait,
		Logger:    state.logger,
		Now:       d.now,
	})
	if err := monitor.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start host load monitor: %w", err)
	}
	cleanup.add(monitor.Shutdown)
	d.admission.SetLoadMonitor(monitor)
	state.runtimeWorkers.hostLoad = monitor
	return nil
}

// hostLoadSnapshotSource keeps an absent monitor a nil interface for doctor wiring.
func hostLoadSnapshotSource(monitor *admission.LoadMonitor) doctor.HostLoadSnapshotSource {
	if monitor == nil {
		return nil
	}
	return monitor
}
