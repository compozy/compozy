package shared

// ProcessResult contains the result of processing a template
type ProcessResult struct {
	Text string
	YAML any
	JSON any
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
