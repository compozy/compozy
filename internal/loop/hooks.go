package loop

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

// HookDispatcher is the loop hook surface consumed at loop-owned call sites.
type HookDispatcher interface {
	DispatchLoopStarted(
		context.Context,
		hookspkg.LoopStartedPayload,
	) (hookspkg.LoopStartedPayload, error)
	DispatchLoopGenerationPre(
		context.Context,
		hookspkg.LoopGenerationPrePayload,
	) (hookspkg.LoopGenerationPrePayload, error)
	DispatchLoopGenerationPost(
		context.Context,
		hookspkg.LoopGenerationPostPayload,
	) (hookspkg.LoopGenerationPostPayload, error)
	DispatchLoopGatePre(
		context.Context,
		hookspkg.LoopGatePrePayload,
	) (hookspkg.LoopGatePrePayload, error)
	DispatchLoopGatePost(
		context.Context,
		hookspkg.LoopGatePostPayload,
	) (hookspkg.LoopGatePostPayload, error)
	DispatchLoopNodeTerminal(
		context.Context,
		hookspkg.LoopNodeTerminalPayload,
	) (hookspkg.LoopNodeTerminalPayload, error)
	DispatchLoopTerminal(
		context.Context,
		hookspkg.LoopTerminalPayload,
	) (hookspkg.LoopTerminalPayload, error)
}
