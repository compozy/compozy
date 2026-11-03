package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/situation"
)

func instanceContextObservation(
	state *bootState,
) func(context.Context, *session.Info) (situation.ContextObservation, error) {
	return func(ctx context.Context, info *session.Info) (situation.ContextObservation, error) {
		reader, ok := state.sessions.(loopSessionEventReader)
		if !ok {
			return situation.ContextObservation{}, nil
		}
		runtime := loopGoalContextRuntime{sessions: reader}
		usage, err := runtime.Usage(ctx, loop.ActionSessionBinding{SessionID: info.ID})
		if err != nil {
			return situation.ContextObservation{}, err
		}
		observation := situation.ContextObservation{
			Known: usage.Known, Used: usage.Used, Size: usage.Size,
			Threshold: state.cfg.Agents.AlwaysOn.RotationThreshold,
		}
		if info.InstanceID == "" || info.Type != session.SessionTypeUser {
			return observation, nil
		}
		provider, ok := state.registry.(instanceRepositoryProvider)
		if !ok {
			return observation, nil
		}
		instance, err := provider.Instances().Get(ctx, info.InstanceID)
		if err != nil {
			return observation, err
		}
		observation.Main = instance.MainSessionID == info.ID
		return observation, nil
	}
}
