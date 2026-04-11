package agent

import (
	"context"

	"github.com/compozy/compozy/internal/core/model"
)

type sessionCreateHookPayload struct {
	RunID          string         `json:"run_id"`
	JobID          string         `json:"job_id"`
	SessionRequest SessionRequest `json:"session_request"`
}

type sessionResumeHookPayload struct {
	RunID         string               `json:"run_id"`
	JobID         string               `json:"job_id"`
	ResumeRequest ResumeSessionRequest `json:"resume_request"`
}

type sessionCreatedHookPayload struct {
	RunID     string          `json:"run_id"`
	JobID     string          `json:"job_id"`
	SessionID string          `json:"session_id"`
	Identity  SessionIdentity `json:"identity"`
}

type SessionOutcome struct {
	Status model.SessionStatus `json:"status"`
	Error  string              `json:"error,omitempty"`
}

func (r SessionRequest) withHookContextFrom(src SessionRequest) SessionRequest {
	r.Context = src.Context
	r.RunID = src.RunID
	r.JobID = src.JobID
	r.RuntimeMgr = src.RuntimeMgr
	return r
}

func (r SessionRequest) context() context.Context {
	if r.Context != nil {
		return r.Context
	}
	return context.Background()
}

func (r SessionRequest) dispatchPreCreateHook() (SessionRequest, error) {
	if r.RuntimeMgr == nil {
		return r, nil
	}

	payload, err := model.DispatchMutableHook(
		r.context(),
		r.RuntimeMgr,
		"agent.pre_session_create",
		sessionCreateHookPayload{
			RunID:          r.RunID,
			JobID:          r.JobID,
			SessionRequest: r,
		},
	)
	if err != nil {
		return SessionRequest{}, err
	}
	return payload.SessionRequest.withHookContextFrom(r), nil
}

func (r ResumeSessionRequest) withHookContextFrom(src ResumeSessionRequest) ResumeSessionRequest {
	r.Context = src.Context
	r.RunID = src.RunID
	r.JobID = src.JobID
	r.RuntimeMgr = src.RuntimeMgr
	return r
}

func (r ResumeSessionRequest) context() context.Context {
	if r.Context != nil {
		return r.Context
	}
	return context.Background()
}

func (r ResumeSessionRequest) dispatchPreResumeHook() (ResumeSessionRequest, error) {
	if r.RuntimeMgr == nil {
		return r, nil
	}

	payload, err := model.DispatchMutableHook(
		r.context(),
		r.RuntimeMgr,
		"agent.pre_session_resume",
		sessionResumeHookPayload{
			RunID:         r.RunID,
			JobID:         r.JobID,
			ResumeRequest: r,
		},
	)
	if err != nil {
		return ResumeSessionRequest{}, err
	}
	return payload.ResumeRequest.withHookContextFrom(r), nil
}

func newSessionOutcome(status model.SessionStatus, err error) SessionOutcome {
	outcome := SessionOutcome{Status: status}
	if err != nil {
		outcome.Error = err.Error()
	}
	return outcome
}
