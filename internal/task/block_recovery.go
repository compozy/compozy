package task

import "context"

func (m *Service) publishAndReconcileRecoveredTask(
	ctx context.Context,
	cleared *NeedsAttentionClearResult,
	actor ActorContext,
) (Task, error) {
	m.OnTaskEvent(ctx, cleared.Event)
	return m.reconcileTaskCascade(ctx, cleared.Task.ID, actor)
}
