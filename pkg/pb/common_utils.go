package pb

import (
	"google.golang.org/protobuf/proto"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

func (x *AgentMetadata) Clone(source string) *AgentMetadata {
	metadataCopy := proto.Clone(x).(*AgentMetadata)
	metadataCopy.Time = timepb.Now()
	metadataCopy.Source = source
	return metadataCopy
}

func (x *TaskMetadata) Clone(source string) *TaskMetadata {
	metadataCopy := proto.Clone(x).(*TaskMetadata)
	metadataCopy.Time = timepb.Now()
	metadataCopy.Source = source
	return metadataCopy
}

func (x *ToolMetadata) Clone(source string) *ToolMetadata {
	metadataCopy := proto.Clone(x).(*ToolMetadata)
	metadataCopy.Time = timepb.Now()
	metadataCopy.Source = source
	return metadataCopy
}

func (x *WorkflowMetadata) Clone(source string) *WorkflowMetadata {
	metadataCopy := proto.Clone(x).(*WorkflowMetadata)
	metadataCopy.Time = timepb.Now()
	metadataCopy.Source = source
	return metadataCopy
}
