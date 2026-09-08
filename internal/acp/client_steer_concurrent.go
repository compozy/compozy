package acp

import (
	"errors"

	acpsdk "github.com/coder/acp-go-sdk"
)

// Called with steerMu held, so the owning prompt cannot begin its final drain
// while this request is being admitted.
func (d *Driver) steerConcurrentPrompt(
	proc *AgentProcess, active *activePromptState, request acpsdk.PromptRequest,
) (SteerResult, error) {
	if active.steerContext == nil {
		return SteerResult{}, ErrSteerTurnMismatch
	}
	run, ok := proc.beginChildTask()
	if !ok {
		return SteerResult{}, errProcessExited
	}
	response, err := acpsdk.SendRequestAsync[wirePromptResponse](
		proc.conn, active.steerContext, acpsdk.AgentMethodSessionPrompt, request,
	)
	if err != nil {
		proc.finishChildTask(run)
		return SteerResult{}, err
	}
	completion := make(chan error, 1)
	active.steerWG.Add(1)
	proc.startReservedChildTask(run, func() {
		defer active.steerWG.Done()
		defer close(completion)
		outcome, open := <-response
		if !open {
			completion <- errors.New("acp: concurrent steer ended without a result")
			return
		}
		completion <- outcome.Err
	})
	return SteerResult{Attempt: SteerAttemptPendingInjection, Completion: completion}, nil
}
