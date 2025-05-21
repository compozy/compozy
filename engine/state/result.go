package state

import (
	"maps"

	"github.com/compozy/compozy/engine/common"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
)

func SetResultData(state *BaseState, res *pbcommon.Result) {
	if res == nil {
		return
	}
	if res.GetError() != nil {
		state.Output = nil
		errorRes := res.GetError()
		state.Error = &Error{
			Message: errorRes.GetMessage(),
		}
		if errorRes.GetCode() != "" {
			state.Error.Code = errorRes.GetCode()
		}
		if errorRes.GetDetails() != nil {
			state.Error.Details = errorRes.GetDetails().AsMap()
		}
	} else if res.GetOutput() != nil {
		state.Error = nil

		output := &common.Output{}
		maps.Copy((*output), res.GetOutput().AsMap())
		state.Output = output
	}
}
