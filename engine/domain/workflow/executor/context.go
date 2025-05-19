package executor

import (
	pb "github.com/compozy/compozy/pkg/pb/workflow"
	"google.golang.org/protobuf/types/known/structpb"
)

func (e *Executor) initContext(cmd *pb.WorkflowExecuteCommand) (*structpb.Struct, error) {
	if cmd.GetPayload() == nil || cmd.GetPayload().GetContext() == nil {
		return structpb.NewStruct(map[string]any{})
	}
	return cmd.GetPayload().GetContext(), nil
}

func createEventContext(cmd *pb.WorkflowExecuteCommand) *structpb.Struct {
	if cmd.GetPayload() == nil || cmd.GetPayload().GetContext() == nil {
		empty, err := structpb.NewStruct(map[string]any{})
		if err != nil {
			return &structpb.Struct{Fields: make(map[string]*structpb.Value)}
		}
		return empty
	}

	// TODO: finish the context here
	contextMap := make(map[string]any)
	result, err := structpb.NewStruct(contextMap)
	if err != nil {
		empty, err := structpb.NewStruct(map[string]any{})
		if err != nil {
			return &structpb.Struct{Fields: make(map[string]*structpb.Value)}
		}
		return empty
	}
	return result
}

func ptr(s string) *string {
	return &s
}
