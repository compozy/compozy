package wfrouter

import "github.com/compozy/compozy/engine/infra/server/router"

// WorkflowDocument models the serialized workflow returned by the HTTP API.
type WorkflowDocument struct {
	ID          string   `json:"id"                    example:"data-processing"`
	Name        string   `json:"name"                  example:"Data Processing Workflow"`
	Description string   `json:"description,omitempty" example:"Ingests raw data and executes nightly transformations."`
	Project     string   `json:"project"               example:"production"`
	Version     string   `json:"version,omitempty"     example:"2025-09-20T12:30:00Z"`
	State       string   `json:"state,omitempty"       example:"active"`
	Tags        []string `json:"tags,omitempty"        example:"nightly,batch"                                          swaggertype:"array,string"`
	TaskIDs     []string `json:"task_ids"              example:"validate-input,transform-data"                          swaggertype:"array,string"`
	AgentIDs    []string `json:"agent_ids"             example:"coordination-agent"                                     swaggertype:"array,string"`
	ToolIDs     []string `json:"tool_ids"              example:"s3-writer"                                              swaggertype:"array,string"`
	TaskCount   int      `json:"task_count"            example:"3"`
	AgentCount  int      `json:"agent_count"           example:"1"`
	ToolCount   int      `json:"tool_count"            example:"2"`
	Tasks       []string `json:"tasks"                 example:"validate-input"                                         swaggertype:"array,string"`
	Agents      []string `json:"agents"                example:"coordination-agent"                                     swaggertype:"array,string"`
	Tools       []string `json:"tools"                 example:"s3-writer"                                              swaggertype:"array,string"`
	ETag        string   `json:"_etag"                 example:"6b1c1d7f448c1c76"`
}

// WorkflowListPageDocument documents the pagination metadata emitted by list endpoints.
type WorkflowListPageDocument struct {
	Limit      int    `json:"limit"                 example:"50"`
	NextCursor string `json:"next_cursor,omitempty" example:"djI6YWZ0ZXI6d29ya2Zsb3ctMDAwMQ=="`
	PrevCursor string `json:"prev_cursor,omitempty" example:"djI6YmVmb3JlOndvcmtmbG93LTAwMDE="`
	Total      int    `json:"total"                 example:"120"`
}

// WorkflowListDocument represents the data payload of the workflow list response.
type WorkflowListDocument struct {
	Workflows []WorkflowDocument       `json:"workflows"`
	Page      WorkflowListPageDocument `json:"page"`
}

// WorkflowProblemExample showcases a standard RFC7807 error envelope for workflows.
type WorkflowProblemExample struct {
	Type     string `json:"type"     example:"https://api.compozy.dev/problems/workflow-not-found"`
	Title    string `json:"title"    example:"Workflow not found"`
	Status   int    `json:"status"   example:"404"`
	Detail   string `json:"detail"   example:"Workflow 'data-processing' was not found in project production."`
	Instance string `json:"instance" example:"/workflows/data-processing"`
}

// WorkflowAcceptedResponse documents the asynchronous execution acknowledgement payload.
type WorkflowAcceptedResponse struct {
	ExecutionID string `json:"execution_id" example:"2Z4PVTL6K27XVT4A3NPKMDD5BG"`
}

// WorkflowExecutionListDocument describes the response body when listing executions.
type WorkflowExecutionListDocument struct {
	Executions []WorkflowExecutionStateDocument `json:"executions"`
}

// WorkflowExecutionStateDocument trims the execution state for documentation purposes.
type WorkflowExecutionStateDocument struct {
	ExecutionID string               `json:"execution_id"          example:"2Z4PVTL6K27XVT4A3NPKMDD5BG"`
	WorkflowID  string               `json:"workflow_id"           example:"data-processing"`
	Status      string               `json:"status"                example:"completed"`
	StartedAt   string               `json:"started_at"            example:"2025-09-20T12:00:00Z"`
	FinishedAt  string               `json:"finished_at,omitempty" example:"2025-09-20T12:05:14Z"`
	Usage       *router.UsageSummary `json:"usage,omitempty"`
}
