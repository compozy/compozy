package nats

import (
	"fmt"
	"reflect"

	"github.com/compozy/compozy/engine/common"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
)

func GetCorrIDFromEvent(evt Event) (common.CorrID, error) {
	if evt == nil {
		return "", fmt.Errorf("event is nil")
	}

	metadata := evt.GetMetadata()
	if metadata == nil {
		return "", fmt.Errorf("event metadata is nil")
	}

	corrID := metadata.GetCorrelationId()
	if corrID == "" {
		return "", fmt.Errorf("correlation ID is empty")
	}

	return common.CorrID(corrID), nil
}

func GetCompIDFromEvent(evt Event, compType ComponentType) (common.CompID, error) {
	if evt == nil {
		return "", fmt.Errorf("event is nil")
	}

	switch compType {
	case ComponentWorkflow:
		workflowInfo := evt.GetWorkflow()
		if workflowInfo == nil {
			return "", fmt.Errorf("workflow info is nil")
		}
		return common.CompID(workflowInfo.GetId()), nil

	case ComponentTask:
		taskInfo := evt.GetTask()
		if taskInfo == nil {
			return "", fmt.Errorf("task info is nil")
		}
		return common.CompID(taskInfo.GetId()), nil

	case ComponentAgent:
		agentInfo := evt.GetAgent()
		if agentInfo == nil {
			return "", fmt.Errorf("agent info is nil")
		}
		return common.CompID(agentInfo.GetId()), nil

	case ComponentTool:
		toolInfo := evt.GetTool()
		if toolInfo == nil {
			return "", fmt.Errorf("tool info is nil")
		}
		return common.CompID(toolInfo.GetId()), nil

	default:
		return "", fmt.Errorf("unsupported component type: %s", compType)
	}
}

func GetExecIDFromEvent(evt Event, compType ComponentType) (common.ExecID, error) {
	if evt == nil {
		return "", fmt.Errorf("event is nil")
	}

	switch compType {
	case ComponentWorkflow:
		workflowInfo := evt.GetWorkflow()
		if workflowInfo == nil {
			return "", fmt.Errorf("workflow info is nil")
		}
		return common.ExecID(workflowInfo.GetExecId()), nil

	case ComponentTask:
		taskInfo := evt.GetTask()
		if taskInfo == nil {
			return "", fmt.Errorf("task info is nil")
		}
		return common.ExecID(taskInfo.GetExecId()), nil

	case ComponentAgent:
		agentInfo := evt.GetAgent()
		if agentInfo == nil {
			return "", fmt.Errorf("agent info is nil")
		}
		return common.ExecID(agentInfo.GetExecId()), nil

	case ComponentTool:
		toolInfo := evt.GetTool()
		if toolInfo == nil {
			return "", fmt.Errorf("tool info is nil")
		}
		return common.ExecID(toolInfo.GetExecId()), nil

	default:
		return "", fmt.Errorf("unsupported component type: %s", compType)
	}
}

func ResultFromEvent(evt Event) (*pbcommon.Result, error) {
	if evt == nil {
		return nil, fmt.Errorf("event is nil")
	}

	payload := evt.GetPayload()
	if payload == nil {
		return nil, fmt.Errorf("event payload is nil")
	}

	payloadValue := reflect.ValueOf(payload)
	resultMethod := payloadValue.MethodByName("GetResult")

	if !resultMethod.IsValid() {
		return nil, fmt.Errorf("payload does not have GetResult method")
	}

	result := resultMethod.Call(nil)[0].Interface()
	if result == nil {
		return nil, fmt.Errorf("result is nil")
	}

	castedResult, ok := result.(*pbcommon.Result)
	if !ok {
		return nil, fmt.Errorf("result is not of type *common.Result")
	}

	return castedResult, nil
}

func StatusFromEvent(evt Event) (EvStatusType, error) {
	if evt == nil {
		return "", fmt.Errorf("event is nil")
	}

	payload := evt.GetPayload()
	if payload == nil {
		return "", fmt.Errorf("event payload is nil")
	}

	payloadValue := reflect.ValueOf(payload)
	statusMethod := payloadValue.MethodByName("GetStatus")

	if !statusMethod.IsValid() {
		payloadType := reflect.TypeOf(payload).String()
		return "", fmt.Errorf("payload type %s does not have GetStatus method", payloadType)
	}

	statusResult := statusMethod.Call(nil)
	if len(statusResult) == 0 {
		return "", fmt.Errorf("GetStatus method returned no value")
	}

	statusValue := statusResult[0].Interface()
	if statusValue == nil {
		return "", fmt.Errorf("status is nil")
	}

	eventStatus, ok := statusValue.(EventStatus)
	if !ok {
		statusType := reflect.TypeOf(statusValue).String()
		return "", fmt.Errorf("status is of type %s, not EventStatus", statusType)
	}

	return ToStatus(eventStatus), nil
}
