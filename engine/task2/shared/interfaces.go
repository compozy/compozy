package shared

// ProcessResult contains the result of processing a template
type ProcessResult struct {
	Text string
	YAML any
	JSON any
}

// TemplateEngine defines the interface for template processing
type TemplateEngine interface {
	Process(template string, vars map[string]any) (string, error)
	ProcessMap(data map[string]any, vars map[string]any) (map[string]any, error)
	ProcessSlice(slice []any, vars map[string]any) ([]any, error)
	ProcessString(templateStr string, context map[string]any) (*ProcessResult, error)
	ParseMapWithFilter(data map[string]any, vars map[string]any, filter func(string) bool) (map[string]any, error)
	ParseMap(data map[string]any, vars map[string]any) (map[string]any, error)
}

// TaskNormalizer defines the contract for task-specific normalization
type TaskNormalizer interface {
	// Normalize applies task-specific normalization rules
	Normalize(config any, ctx *NormalizationContext) error
	// Type returns the task type this normalizer handles
	Type() string
}

// NormalizerFactory creates appropriate normalizers
type NormalizerFactory interface {
	CreateNormalizer(taskType string) (TaskNormalizer, error)
}
