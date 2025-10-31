package client

import (
	"time"

	agentrouter "github.com/compozy/compozy/engine/agent/router"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	wfrouter "github.com/compozy/compozy/engine/workflow/router"
)

// WorkflowExecuteRequest represents the async workflow execution payload.
type WorkflowExecuteRequest = wfrouter.ExecuteWorkflowRequest

// WorkflowExecuteResponse mirrors the engine async workflow response.
type WorkflowExecuteResponse = wfrouter.ExecuteWorkflowResponse

// WorkflowSyncRequest mirrors the workflow sync request payload.
type WorkflowSyncRequest = wfrouter.WorkflowSyncRequest

// WorkflowSyncResponse mirrors the workflow sync response payload.
type WorkflowSyncResponse = wfrouter.WorkflowSyncResponse

// TaskExecuteRequest represents direct task execution input.
type TaskExecuteRequest = tkrouter.TaskExecRequest

// TaskExecuteResponse mirrors the async task execution response.
type TaskExecuteResponse = tkrouter.TaskExecAsyncResponse

// TaskSyncResponse mirrors the synchronous task execution response.
type TaskSyncResponse = tkrouter.TaskExecSyncResponse

// AgentExecuteRequest represents agent execution input.
type AgentExecuteRequest = agentrouter.AgentExecRequest

// AgentExecuteResponse mirrors the async agent execution response.
type AgentExecuteResponse = agentrouter.AgentExecAsyncResponse

// AgentSyncResponse mirrors the synchronous agent execution response.
type AgentSyncResponse = agentrouter.AgentExecSyncResponse

// StreamOptions customizes streaming behaviour for workflow, task, or agent execution streams.
type StreamOptions struct {
	// PollInterval controls the poll_ms query parameter.
	PollInterval time.Duration
	// Events filters emitted event types using the events query parameter.
	Events []string
	// LastEventID resumes streaming from a specific event id via Last-Event-ID header.
	LastEventID *int64
}

// StreamEvent encapsulates a single SSE frame from execution streams.
type StreamEvent struct {
	ID   int64
	Type string
	Data []byte
}
