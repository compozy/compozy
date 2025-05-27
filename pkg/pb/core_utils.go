package pb

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"google.golang.org/protobuf/proto"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

// -----------------------------------------------------------------------------
// Logging
// -----------------------------------------------------------------------------

func (x *AgentMetadata) Logger() *logger.Logger {
	return logger.With(
		"workflow_id", x.WorkflowId,
		"workflow_exec_id", x.WorkflowExecId,
		"agent_id", x.AgentId,
		"agent_exec_id", x.AgentExecId,
		"task_id", x.TaskId,
		"task_exec_id", x.TaskExecId,
	)
}

func (x *TaskMetadata) Logger() *logger.Logger {
	return logger.With(
		"workflow_id", x.WorkflowId,
		"workflow_exec_id", x.WorkflowExecId,
		"task_id", x.TaskId,
		"task_exec_id", x.TaskExecId,
	)
}

func (x *ToolMetadata) Logger() *logger.Logger {
	return logger.With(
		"workflow_id", x.WorkflowId,
		"workflow_exec_id", x.WorkflowExecId,
		"task_id", x.TaskId,
		"task_exec_id", x.TaskExecId,
		"tool_id", x.ToolId,
		"tool_exec_id", x.ToolExecId,
	)
}

func (x *WorkflowMetadata) Logger() *logger.Logger {
	return logger.With(
		"workflow_id", x.WorkflowId,
		"workflow_exec_id", x.WorkflowExecId,
	)
}

// -----------------------------------------------------------------------------
// Clone
// -----------------------------------------------------------------------------

func (x *AgentMetadata) Clone(source core.SourceType) (*AgentMetadata, error) {
	metadataCopy, ok := proto.Clone(x).(*AgentMetadata)
	if !ok {
		return nil, fmt.Errorf("failed to clone AgentMetadata")
	}
	metadataCopy.Time = timepb.Now()
	metadataCopy.Source = source.String()
	// Ensure version is set
	if metadataCopy.Version == "" {
		metadataCopy.Version = core.GetVersion()
	}
	return metadataCopy, nil
}

func (x *AgentMetadata) MustClone(source core.SourceType) *AgentMetadata {
	metadata, err := x.Clone(source)
	if err != nil {
		panic(err)
	}
	return metadata
}

func (x *TaskMetadata) Clone(source core.SourceType) (*TaskMetadata, error) {
	metadataCopy, ok := proto.Clone(x).(*TaskMetadata)
	if !ok {
		return nil, fmt.Errorf("failed to clone TaskMetadata")
	}
	metadataCopy.Time = timepb.Now()
	metadataCopy.Source = source.String()
	// Ensure version is set
	if metadataCopy.Version == "" {
		metadataCopy.Version = core.GetVersion()
	}
	return metadataCopy, nil
}

func (x *TaskMetadata) MustClone(source core.SourceType) *TaskMetadata {
	metadata, err := x.Clone(source)
	if err != nil {
		panic(err)
	}
	return metadata
}

func (x *ToolMetadata) Clone(source core.SourceType) (*ToolMetadata, error) {
	metadataCopy, ok := proto.Clone(x).(*ToolMetadata)
	if !ok {
		return nil, fmt.Errorf("failed to clone ToolMetadata")
	}
	metadataCopy.Time = timepb.Now()
	metadataCopy.Source = source.String()
	// Ensure version is set
	if metadataCopy.Version == "" {
		metadataCopy.Version = core.GetVersion()
	}
	return metadataCopy, nil
}

func (x *ToolMetadata) MustClone(source core.SourceType) *ToolMetadata {
	metadata, err := x.Clone(source)
	if err != nil {
		panic(err)
	}
	return metadata
}

func (x *WorkflowMetadata) Clone(source core.SourceType) (*WorkflowMetadata, error) {
	metadataCopy, ok := proto.Clone(x).(*WorkflowMetadata)
	if !ok {
		return nil, fmt.Errorf("failed to clone WorkflowMetadata")
	}
	metadataCopy.Time = timepb.Now()
	metadataCopy.Source = source.String()
	// Ensure version is set
	if metadataCopy.Version == "" {
		metadataCopy.Version = core.GetVersion()
	}
	return metadataCopy, nil
}

func (x *WorkflowMetadata) MustClone(source core.SourceType) *WorkflowMetadata {
	metadata, err := x.Clone(source)
	if err != nil {
		panic(err)
	}
	return metadata
}
