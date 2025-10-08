package toolcontext

import "context"

type ctxKey string

const (
	plannerDisabledKey   ctxKey = "toolcontext.planner_disabled"
	orchestratorDepthKey ctxKey = "toolcontext.agent_orchestrator_depth"
)

func DisablePlannerTools(ctx context.Context) context.Context {
	if ctx == nil {
		return nil
	}
	return context.WithValue(ctx, plannerDisabledKey, true)
}

func PlannerToolsDisabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	disabled, ok := ctx.Value(plannerDisabledKey).(bool)
	if !ok {
		return false
	}
	return disabled
}

func AgentOrchestratorDepth(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	depth, ok := ctx.Value(orchestratorDepthKey).(int)
	if !ok {
		return 0
	}
	if depth < 0 {
		return 0
	}
	return depth
}

func IncrementAgentOrchestratorDepth(ctx context.Context) context.Context {
	if ctx == nil {
		return nil
	}
	depth := AgentOrchestratorDepth(ctx)
	return context.WithValue(ctx, orchestratorDepthKey, depth+1)
}
