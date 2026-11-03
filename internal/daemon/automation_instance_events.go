package daemon

import (
	"context"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type automationInstanceEvents struct {
	sink    automationpkg.InstanceEventSink
	enabled bool
}

var _ automationpkg.InstanceEventSink = (*automationInstanceEvents)(nil)

func automationInstanceEventSink(state *bootState) automationpkg.InstanceEventSink {
	provider, ok := state.registry.(interface {
		InstanceEvents(time.Duration) *globaldb.InstanceEventRepo
	})
	if !ok {
		return nil
	}
	return &automationInstanceEvents{
		sink:    provider.InstanceEvents(state.cfg.Agents.Heartbeat.MinInterval),
		enabled: state.cfg.Agents.Heartbeat.Enabled,
	}
}

func (s *automationInstanceEvents) Deliver(
	ctx context.Context,
	target automationpkg.InstanceTarget,
	event automationpkg.InstanceEvent,
) (automationpkg.InstanceEventDelivery, error) {
	target.ChecksSupported = target.ChecksSupported && s.enabled
	result, err := s.sink.Deliver(ctx, target, event)
	if err == nil && !s.enabled {
		result.CheckScheduled, result.Reason = false, "heartbeat_disabled"
	}
	return result, err
}
