package doctor

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	SubprocessHealthProbeID      = "runtime.subprocess_health"
	maxSubprocessHealthReasonLen = 1024
)

// SubprocessHealthSnapshotSource returns immutable active-process health observations.
type SubprocessHealthSnapshotSource interface {
	SubprocessHealthSnapshots() []session.SubprocessHealthSnapshot
}

// SubprocessHealthProbe projects failed active-process health verdicts into doctor diagnostics.
type SubprocessHealthProbe struct {
	Source SubprocessHealthSnapshotSource
}

var _ Probe = (*SubprocessHealthProbe)(nil)

func (*SubprocessHealthProbe) ID() string { return SubprocessHealthProbeID }

func (*SubprocessHealthProbe) Category() string { return contract.CategoryDaemon }

func (p *SubprocessHealthProbe) Run(
	_ context.Context,
	_ *ProbeEnv,
) ([]contract.DiagnosticItem, error) {
	if p == nil || p.Source == nil {
		return []contract.DiagnosticItem{subprocessHealthUnavailableItem()}, nil
	}

	snapshots := p.Source.SubprocessHealthSnapshots()
	sort.Slice(snapshots, func(i int, j int) bool {
		return snapshots[i].SessionID < snapshots[j].SessionID
	})
	failed := make([]session.SubprocessHealthSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if subprocess.HealthFailureDetected(snapshot.Health) {
			failed = append(failed, snapshot)
		}
	}
	if len(failed) == 0 {
		return []contract.DiagnosticItem{diagnostics.NewItem(
			SubprocessHealthProbeID,
			contract.CodeDaemonStatusOK,
			contract.CategoryDaemon,
			"Subprocess health is OK",
			"No active subprocess reports a failed health verdict.",
			contract.SeverityOK,
			contract.FreshnessLive,
			diagnostics.WithEvidence(map[string]any{"monitored_count": len(snapshots)}),
		)}, nil
	}

	first := failed[0]
	reason := diagnostics.RedactAndBound(
		subprocess.HealthFailureReason(first.Health),
		maxSubprocessHealthReasonLen,
	)
	if strings.TrimSpace(reason) == "" {
		reason = "the subprocess reported an unhealthy verdict"
	}
	return []contract.DiagnosticItem{diagnostics.NewItem(
		SubprocessHealthProbeID,
		contract.CodeDaemonHealthUnavailable,
		contract.CategoryDaemon,
		"Subprocess health is degraded",
		reason,
		contract.SeverityWarn,
		contract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{
			"session_id":           strings.TrimSpace(first.SessionID),
			"workspace_id":         strings.TrimSpace(first.WorkspaceID),
			"agent_name":           strings.TrimSpace(first.AgentName),
			"last_checked_at":      subprocessHealthTimestamp(first.Health.LastCheckedAt),
			"consecutive_failures": first.Health.ConsecutiveFailures,
			"monitored_count":      len(snapshots),
			"unhealthy_count":      len(failed),
		}),
	)}, nil
}

func subprocessHealthUnavailableItem() contract.DiagnosticItem {
	return diagnostics.NewItem(
		SubprocessHealthProbeID,
		contract.CodeDaemonHealthUnavailable,
		contract.CategoryDaemon,
		"Subprocess health is unavailable",
		"The session runtime does not expose subprocess health snapshots.",
		contract.SeverityWarn,
		contract.FreshnessLive,
	)
}

func subprocessHealthTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
