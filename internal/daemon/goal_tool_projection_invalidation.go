package daemon

import (
	"context"
	"strings"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
)

// Goal tool availability (goal_get, goal_report) follows Goal run, binding, and prompt state that
// lives outside the tool registry. These hooks advance the tool projection epoch at the owning
// transitions so hosted MCP streams and cached Goal availability refresh immediately.

// goalToolProjectionInvalidator returns the epoch advance for Goal transitions, or nil when the
// daemon has no projection epoch.
func goalToolProjectionInvalidator(state *bootState) func() {
	if state == nil || state.toolProjectionEpoch == nil {
		return nil
	}
	epoch := state.toolProjectionEpoch
	return func() { epoch.Advance() }
}

// goalCommandChangesToolState reports whether a Goal command can create, clear, or move a Goal.
func goalCommandChangesToolState(verb string) bool {
	switch strings.TrimSpace(verb) {
	case session.GoalCommandVerbSet,
		session.GoalCommandVerbReplace,
		session.GoalCommandVerbPause,
		session.GoalCommandVerbResume,
		session.GoalCommandVerbStop, session.GoalCommandVerbClear:
		return true
	default:
		return false
	}
}

func (s *daemonLoopAPIService) notifyGoalToolsChanged() {
	if s != nil && s.goalToolsChanged != nil {
		s.goalToolsChanged()
	}
}

// goalToolProjectionObserver advances the epoch when a Loop run starts or reaches a terminal
// state, which is when active Goal report targets and moved-binding aliases appear or vanish.
type goalToolProjectionObserver struct {
	invalidate func()
}

var _ loopStartedObserver = goalToolProjectionObserver{}
var _ loopTerminalObserver = goalToolProjectionObserver{}

func (o goalToolProjectionObserver) OnLoopStarted(context.Context, hookspkg.LoopStartedPayload) error {
	o.invalidate()
	return nil
}

func (o goalToolProjectionObserver) OnLoopTerminal(context.Context, hookspkg.LoopTerminalPayload) error {
	o.invalidate()
	return nil
}

func installGoalToolProjectionObserver(state *bootState) {
	invalidate := goalToolProjectionInvalidator(state)
	if invalidate == nil || state.notifier == nil {
		return
	}
	observer := goalToolProjectionObserver{invalidate: invalidate}
	state.notifier.AddLoopStartedObserver(observer)
	state.notifier.AddLoopTerminalObserver(observer)
}

// goalToolProjectionRuntime decorates the Goal runtime so prompt and binding transitions advance
// the projection epoch: goal_report is available exactly while a Goal prompt is bound.
type goalToolProjectionRuntime struct {
	*loopGoalRuntime
	invalidate func()
}

var _ loopGoalRuntimePorts = goalToolProjectionRuntime{}
var _ looppkg.ActionSessionBinder = goalToolProjectionRuntime{}

// withGoalToolProjection returns runtime unchanged when no invalidator is wired.
func withGoalToolProjection(runtime *loopGoalRuntime, invalidate func()) loopGoalBinderRuntime {
	if invalidate == nil {
		return runtime
	}
	return goalToolProjectionRuntime{loopGoalRuntime: runtime, invalidate: invalidate}
}

// loopGoalBinderRuntime is the Goal runtime surface consumed by the Loop action registry.
type loopGoalBinderRuntime interface {
	loopGoalRuntimePorts
	looppkg.ActionSessionBinder
}

func (r goalToolProjectionRuntime) BindActionSession(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
) (looppkg.ActionSessionBinding, error) {
	binding, err := r.loopGoalRuntime.BindActionSession(ctx, req)
	r.invalidate()
	return binding, err
}

func (r goalToolProjectionRuntime) PrepareActionPrompt(
	ctx context.Context,
	binding looppkg.ActionSessionBinding,
	req looppkg.ActionPromptRequest,
) (looppkg.ActionPromptTicket, error) {
	ticket, err := r.loopGoalRuntime.PrepareActionPrompt(ctx, binding, req)
	r.invalidate()
	return ticket, err
}

func (r goalToolProjectionRuntime) AwaitActionPrompt(
	ctx context.Context,
	ticket looppkg.ActionPromptTicket,
) (looppkg.ActionPromptResult, error) {
	result, err := r.loopGoalRuntime.AwaitActionPrompt(ctx, ticket)
	r.invalidate()
	return result, err
}
