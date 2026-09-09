package doctor

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	loggerpkg "github.com/compozy/compozy/internal/logger"
)

const SessionMetadataProbeID = "runtime.session_metadata"

type SessionMetadataHealthSource interface {
	SessionMetadataHealth(context.Context) (int, loggerpkg.FailureSummary, error)
}

type SessionMetadataProbe struct {
	Source SessionMetadataHealthSource
}

var _ Probe = (*SessionMetadataProbe)(nil)

func (*SessionMetadataProbe) ID() string       { return SessionMetadataProbeID }
func (*SessionMetadataProbe) Category() string { return contract.CategoryDaemon }

func (p *SessionMetadataProbe) Run(ctx context.Context, _ *ProbeEnv) ([]contract.DiagnosticItem, error) {
	checked, failures, err := p.Source.SessionMetadataHealth(ctx)
	if err != nil {
		return nil, err
	}
	spec := diagnostics.ItemSpec{
		ID: SessionMetadataProbeID, Code: contract.CodeDaemonStatusOK, Category: contract.CategoryDaemon,
		Title: "Session metadata is readable", Message: fmt.Sprintf("Checked %d persisted sessions.", checked),
		Severity: contract.SeverityOK, DataFreshness: contract.FreshnessLive,
	}
	if failures.Count > 0 {
		spec.Code = contract.CodeDaemonHealthUnavailable
		spec.Severity = contract.SeverityWarn
		spec.Title = "Unreadable session metadata"
		spec.Message = fmt.Sprintf("%d of %d persisted sessions have unreadable metadata or creation witnesses. "+
			"Preserve the session files and database; inspect the sampled errors before repairing or upgrading.",
			failures.Count, checked)
	}
	return []contract.DiagnosticItem{diagnostics.NewItem(spec, diagnostics.WithEvidence(map[string]any{
		"checked_count": checked, "unreadable_count": failures.Count,
		"samples": failures.Samples, "omitted_count": failures.Count - len(failures.Samples),
	}))}, nil
}
