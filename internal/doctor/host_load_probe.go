package doctor

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
)

const HostLoadProbeID = "runtime.load"

// HostLoadSnapshotSource returns the daemon's latest host CPU pressure observation.
type HostLoadSnapshotSource interface {
	Snapshot() admission.LoadSnapshot
}

// HostLoadProbe projects load-aware admission state into doctor diagnostics.
type HostLoadProbe struct {
	Source HostLoadSnapshotSource
}

var _ Probe = (*HostLoadProbe)(nil)

func (*HostLoadProbe) ID() string { return HostLoadProbeID }

func (*HostLoadProbe) Category() string { return contract.CategoryDaemon }

func (p *HostLoadProbe) Run(_ context.Context, _ *ProbeEnv) ([]contract.DiagnosticItem, error) {
	if p.Source == nil {
		return nil, nil
	}
	snapshot := p.Source.Snapshot()
	spec := diagnostics.ItemSpec{
		ID:            HostLoadProbeID,
		Code:          contract.CodeDaemonStatusOK,
		Category:      contract.CategoryDaemon,
		Title:         "Host CPU load is within the admission threshold",
		Message:       "Background sessions and prompts start without delay.",
		Severity:      contract.SeverityOK,
		DataFreshness: contract.FreshnessLive,
	}
	switch {
	case !snapshot.Enabled:
		spec.Title = "Load-aware admission is disabled"
		spec.Message = "Set daemon.load.admission_threshold above zero and restart the daemon to enable it."
	case !snapshot.Supported:
		spec.Title = "Load-aware admission is unavailable"
		spec.Message = "This platform does not report a load average; background work is not delayed."
	case snapshot.Pressured:
		spec.Title = "Host CPU is saturated; background work is delayed"
		spec.Message = "New automation, Loop, and scheduler sessions and prompts wait up to " +
			"daemon.load.admission_max_wait for load to drop. User prompts are not delayed."
		spec.Severity = contract.SeverityWarn
	}
	return []contract.DiagnosticItem{diagnostics.NewItem(spec,
		diagnostics.WithEvidence(hostLoadEvidence(snapshot)),
	)}, nil
}

func hostLoadEvidence(snapshot admission.LoadSnapshot) map[string]any {
	sampledAt := ""
	if !snapshot.SampledAt.IsZero() {
		sampledAt = snapshot.SampledAt.UTC().Format(time.RFC3339Nano)
	}
	return map[string]any{
		"enabled":         snapshot.Enabled,
		"supported":       snapshot.Supported,
		"pressured":       snapshot.Pressured,
		"threshold":       snapshot.Threshold,
		"max_wait":        snapshot.MaxWait.String(),
		"logical_cpus":    snapshot.LogicalCPUs,
		"load1":           snapshot.Load1,
		"normalized_load": snapshot.NormalizedLoad,
		"sampled_at":      sampledAt,
		"waiting":         snapshot.Waiting,
		"deferred_total":  snapshot.Deferred,
		"wait_expired":    snapshot.WaitExpired,
	}
}
