package pb

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

type SourceType string

const (
	SourceTypeOrchestrator     SourceType = "orchestrator.Orchestrator"
	SourceTypeWorkflowExecutor SourceType = "workflow.Executor"
	SourceTypeTaskExecutor     SourceType = "task.Executor"
	SourceTypeAgentExecutor    SourceType = "agent.Executor"
	SourceTypeToolExecutor     SourceType = "tool.Executor"
)

func (s SourceType) String() string {
	return string(s)
}

func (x *AgentMetadata) Clone(source SourceType) (*AgentMetadata, error) {
	metadataCopy, ok := proto.Clone(x).(*AgentMetadata)
	if !ok {
		return nil, fmt.Errorf("failed to clone AgentMetadata")
	}
	metadataCopy.Time = timepb.Now()
	metadataCopy.Source = source.String()
	return metadataCopy, nil
}

func (x *TaskMetadata) Clone(source SourceType) (*TaskMetadata, error) {
	metadataCopy, ok := proto.Clone(x).(*TaskMetadata)
	if !ok {
		return nil, fmt.Errorf("failed to clone TaskMetadata")
	}
	metadataCopy.Time = timepb.Now()
	metadataCopy.Source = source.String()
	return metadataCopy, nil
}

func (x *ToolMetadata) Clone(source SourceType) (*ToolMetadata, error) {
	metadataCopy, ok := proto.Clone(x).(*ToolMetadata)
	if !ok {
		return nil, fmt.Errorf("failed to clone ToolMetadata")
	}
	metadataCopy.Time = timepb.Now()
	metadataCopy.Source = source.String()
	return metadataCopy, nil
}

func (x *WorkflowMetadata) Clone(source SourceType) (*WorkflowMetadata, error) {
	metadataCopy, ok := proto.Clone(x).(*WorkflowMetadata)
	if !ok {
		return nil, fmt.Errorf("failed to clone WorkflowMetadata")
	}
	metadataCopy.Time = timepb.Now()
	metadataCopy.Source = source.String()
	return metadataCopy, nil
}
