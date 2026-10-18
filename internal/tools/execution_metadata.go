package tools

// ToolExecutionMetadata holds optional dispatch behavior without growing copied descriptors.
type ToolExecutionMetadata struct {
	MaxResultBytes int64     `json:"max_result_bytes,omitempty"`
	Idempotent     bool      `json:"idempotent,omitempty"`
	InputErrorCode ErrorCode `json:"input_error_code,omitempty"`
}

func NewToolExecutionMetadata(idempotent bool, inputErrorCode ErrorCode, maxResultBytes int64) *ToolExecutionMetadata {
	if !idempotent && inputErrorCode == "" && maxResultBytes == 0 {
		return nil
	}
	return &ToolExecutionMetadata{
		Idempotent:     idempotent,
		InputErrorCode: inputErrorCode,
		MaxResultBytes: maxResultBytes,
	}
}

func CloneToolExecutionMetadata(metadata *ToolExecutionMetadata) *ToolExecutionMetadata {
	if metadata == nil {
		return nil
	}
	return new(*metadata)
}

// ExecutionMetadata returns a nil-safe copy of the optional dispatch metadata.
func (d Descriptor) ExecutionMetadata() ToolExecutionMetadata {
	if d.ToolExecutionMetadata == nil {
		return ToolExecutionMetadata{}
	}
	return *d.ToolExecutionMetadata
}

func (t Tool) ExecutionMetadata() ToolExecutionMetadata {
	if t.ToolExecutionMetadata == nil {
		return ToolExecutionMetadata{}
	}
	return *t.ToolExecutionMetadata
}

func (d *Descriptor) SetMaxResultBytes(limit int64) {
	metadata := d.ExecutionMetadata()
	metadata.MaxResultBytes = limit
	d.ToolExecutionMetadata = &metadata
}
