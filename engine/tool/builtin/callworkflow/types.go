package callworkflow

// handlerInput represents the payload accepted by cp__call_workflow.
type handlerInput struct {
	WorkflowID    string         `json:"workflow_id"     mapstructure:"workflow_id"`
	Input         map[string]any `json:"input"           mapstructure:"input"`
	InitialTaskID string         `json:"initial_task_id" mapstructure:"initial_task_id"`
	TimeoutMs     int            `json:"timeout_ms"      mapstructure:"timeout_ms"`
}
