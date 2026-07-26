package daemon

import (
	"log/slog"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/soul"
)

func soulAuthoringServiceDependency(store any, logger *slog.Logger) *soul.ManagedSoulAuthoringService {
	authoringStore, ok := store.(soul.AuthoringStore)
	if !ok {
		return nil
	}
	service, err := soul.NewManagedSoulAuthoringService(authoringStore)
	if err != nil {
		logAuthoredContextDependencyError(logger, "daemon: create soul authoring service", err)
		return nil
	}
	return service
}

func heartbeatAuthoringServiceDependency(store any, logger *slog.Logger) *heartbeat.ManagedHeartbeatAuthoringService {
	authoringStore, ok := store.(heartbeat.AuthoringStore)
	if !ok {
		return nil
	}
	service, err := heartbeat.NewManagedHeartbeatAuthoringService(authoringStore)
	if err != nil {
		logAuthoredContextDependencyError(logger, "daemon: create heartbeat authoring service", err)
		return nil
	}
	return service
}
