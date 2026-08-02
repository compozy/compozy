package doctor

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
)

const RuntimeMemoryProbeID = "runtime.memory"

// RuntimeMemorySnapshot is one immutable daemon process-memory observation.
type RuntimeMemorySnapshot struct {
	Enabled                 bool
	Active                  bool
	CapturedAt              time.Time
	Phase                   string
	ResidentMemoryBytes     uint64
	ResidentMemoryKind      string
	ResidentMemoryAvailable bool
	HeapAllocBytes          uint64
	HeapInUseBytes          uint64
	Goroutines              int
	UptimeSeconds           int64
}

// RuntimeMemorySnapshotSource returns the daemon's latest process-memory observation.
type RuntimeMemorySnapshotSource interface {
	RuntimeMemorySnapshot() RuntimeMemorySnapshot
}

// RuntimeMemoryProbe projects daemon-owned process memory into doctor diagnostics.
type RuntimeMemoryProbe struct {
	Source RuntimeMemorySnapshotSource
}

var _ Probe = (*RuntimeMemoryProbe)(nil)

func (*RuntimeMemoryProbe) ID() string { return RuntimeMemoryProbeID }

func (*RuntimeMemoryProbe) Category() string { return contract.CategoryDaemon }

func (p *RuntimeMemoryProbe) Run(
	_ context.Context,
	_ *ProbeEnv,
) ([]contract.DiagnosticItem, error) {
	if p.Source == nil {
		return []contract.DiagnosticItem{runtimeMemoryUnavailableItem()}, nil
	}

	snapshot := p.Source.RuntimeMemorySnapshot()
	if !snapshot.Enabled {
		return []contract.DiagnosticItem{diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            RuntimeMemoryProbeID,
			Code:          contract.CodeDaemonStatusOK,
			Category:      contract.CategoryDaemon,
			Title:         "Runtime memory reporting is disabled",
			Message:       "Set daemon.memory_report_interval above zero and restart the daemon to enable reports.",
			Severity:      contract.SeverityOK,
			DataFreshness: contract.FreshnessLive,
		},
			diagnostics.WithEvidence(runtimeMemoryEvidence(snapshot)),
		)}, nil
	}
	if snapshot.CapturedAt.IsZero() {
		return []contract.DiagnosticItem{runtimeMemoryUnavailableItem()}, nil
	}

	return []contract.DiagnosticItem{diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            RuntimeMemoryProbeID,
		Code:          contract.CodeDaemonStatusOK,
		Category:      contract.CategoryDaemon,
		Title:         "Runtime memory snapshot is available",
		Message:       "Compozy is reporting process memory and Go runtime utilization.",
		Severity:      contract.SeverityOK,
		DataFreshness: contract.FreshnessLive,
	},
		diagnostics.WithEvidence(runtimeMemoryEvidence(snapshot)),
	)}, nil
}

func runtimeMemoryUnavailableItem() contract.DiagnosticItem {
	return diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            RuntimeMemoryProbeID,
		Code:          contract.CodeDaemonHealthUnavailable,
		Category:      contract.CategoryDaemon,
		Title:         "Runtime memory snapshot is unavailable",
		Message:       "The daemon has not produced a runtime memory snapshot.",
		Severity:      contract.SeverityWarn,
		DataFreshness: contract.FreshnessLive,
	})
}

func runtimeMemoryEvidence(snapshot RuntimeMemorySnapshot) map[string]any {
	capturedAt := ""
	if !snapshot.CapturedAt.IsZero() {
		capturedAt = snapshot.CapturedAt.UTC().Format(time.RFC3339Nano)
	}
	return map[string]any{
		"enabled":                   snapshot.Enabled,
		"active":                    snapshot.Active,
		"captured_at":               capturedAt,
		"phase":                     snapshot.Phase,
		"resident_memory_bytes":     snapshot.ResidentMemoryBytes,
		"resident_memory_kind":      snapshot.ResidentMemoryKind,
		"resident_memory_available": snapshot.ResidentMemoryAvailable,
		"heap_alloc_bytes":          snapshot.HeapAllocBytes,
		"heap_in_use_bytes":         snapshot.HeapInUseBytes,
		"goroutines":                snapshot.Goroutines,
		"uptime_seconds":            snapshot.UptimeSeconds,
	}
}
