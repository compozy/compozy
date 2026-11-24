package tools

// ToolExecutionMetadata holds optional dispatch behavior without growing copied descriptors.
type ToolExecutionMetadata struct {
	OutputViews                *ResultViews `json:"output_views,omitempty"`
	ReservedWorkspacePartition bool         `json:"reserved_workspace_partition,omitempty"`
	MaxResultBytes             int64        `json:"max_result_bytes,omitempty"`
	Idempotent                 bool         `json:"idempotent,omitempty"`
	InputErrorCode             ErrorCode    `json:"input_error_code,omitempty"`
}

func NewToolExecutionMetadata(
	idempotent bool,
	inputErrorCode ErrorCode,
	maxResultBytes int64,
	views ...*ResultViews,
) *ToolExecutionMetadata {
	var outputViews *ResultViews
	if len(views) > 0 {
		outputViews = CloneResultViews(views[0])
	}
	if !idempotent && inputErrorCode == "" && maxResultBytes == 0 && outputViews == nil {
		return nil
	}
	return &ToolExecutionMetadata{
		OutputViews:    outputViews,
		Idempotent:     idempotent,
		InputErrorCode: inputErrorCode,
		MaxResultBytes: maxResultBytes,
	}
}

func CloneToolExecutionMetadata(metadata *ToolExecutionMetadata) *ToolExecutionMetadata {
	if metadata == nil {
		return nil
	}
	cloned := *metadata
	cloned.OutputViews = CloneResultViews(metadata.OutputViews)
	return &cloned
}

// ExecutionMetadata returns a nil-safe copy of the optional dispatch metadata.
func (d Descriptor) ExecutionMetadata() ToolExecutionMetadata {
	if d.ToolExecutionMetadata == nil {
		return ToolExecutionMetadata{}
	}
	return *CloneToolExecutionMetadata(d.ToolExecutionMetadata)
}

func (t Tool) ExecutionMetadata() ToolExecutionMetadata {
	if t.ToolExecutionMetadata == nil {
		return ToolExecutionMetadata{}
	}
	return *CloneToolExecutionMetadata(t.ToolExecutionMetadata)
}

func (d *Descriptor) SetMaxResultBytes(limit int64) {
	metadata := d.ExecutionMetadata()
	metadata.MaxResultBytes = limit
	d.ToolExecutionMetadata = &metadata
}

// ResultViews returns an isolated, nil-safe copy of the declared view contracts.
func (d Descriptor) ResultViews() *ResultViews {
	if d.ToolExecutionMetadata == nil {
		return nil
	}
	return CloneResultViews(d.OutputViews)
}

func (t Tool) ResultViews() *ResultViews {
	if t.ToolExecutionMetadata == nil {
		return nil
	}
	return CloneResultViews(t.OutputViews)
}

func (d *Descriptor) SetResultViews(views *ResultViews) {
	metadata := d.ExecutionMetadata()
	metadata.OutputViews = CloneResultViews(views)
	d.ToolExecutionMetadata = &metadata
}
