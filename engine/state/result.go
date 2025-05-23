package state

import (
	"maps"

	"github.com/compozy/compozy/engine/common"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	"google.golang.org/protobuf/types/known/structpb"
)

func SetStateError(state *BaseState, err *pbcommon.ErrorResult) {
	if err == nil {
		return
	}
	state.Output = nil
	state.Error = &Error{
		Message: err.GetMessage(),
	}
	if err.GetCode() != "" {
		state.Error.Code = err.GetCode()
	}
	if err.GetDetails() != nil {
		state.Error.Details = err.GetDetails().AsMap()
	}
}

func SetStateResult(state *BaseState, output *structpb.Struct) {
	if output == nil {
		return
	}
	state.Error = nil
	res := &common.Output{}
	maps.Copy((*res), output.AsMap())
	state.Output = res
}
