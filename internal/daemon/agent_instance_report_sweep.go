package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type instanceReportRepositoryProvider interface {
	InstanceReports() *globaldb.InstanceReportRepo
}

func sweepInstanceReports(ctx context.Context, state *bootState) error {
	provider, ok := state.registry.(instanceReportRepositoryProvider)
	if !ok {
		return nil
	}
	writer, ok := state.sessions.(instanceReportSessionWriter)
	if !ok {
		return nil
	}
	publisher := agentinstance.ReportPublisher{
		Store:     provider.InstanceReports(),
		Writer:    instanceReportEventWriter{sessions: writer},
		BatchSize: 100,
	}
	return publisher.Sweep(ctx)
}
