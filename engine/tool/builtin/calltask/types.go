package calltask

// handlerInput captures the payload accepted by cp__call_task.
type handlerInput struct {
	TaskID    string         `json:"task_id"    mapstructure:"task_id"`
	With      map[string]any `json:"with"       mapstructure:"with"`
	TimeoutMs int            `json:"timeout_ms" mapstructure:"timeout_ms"`
}
