package core

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
)

// DaemonDrainController owns daemon-global admission and quiesce truth.
type DaemonDrainController interface {
	Drain(ctx context.Context) error
	Undrain(ctx context.Context) error
	DrainState() contract.DrainState
	DrainStatus(ctx context.Context) (contract.DrainStatusResponse, error)
}

// DaemonReadinessReporter is the optional lifecycle-phase readout a drain controller may provide.
type DaemonReadinessReporter interface {
	DaemonReadiness() *contract.DaemonReadinessPayload
}
