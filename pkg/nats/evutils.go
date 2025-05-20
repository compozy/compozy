package nats

import (
	"fmt"
	"reflect"

	"github.com/compozy/compozy/pkg/pb/common"
)

func GetCorrIDFromEvent(event Event) (string, error) {
	if event == nil {
		return "", fmt.Errorf("event is nil")
	}

	metadata := event.GetMetadata()
	if metadata == nil {
		return "", fmt.Errorf("event metadata is nil")
	}

	corrID := metadata.GetCorrelationId()
	if corrID == "" {
		return "", fmt.Errorf("correlation ID is empty")
	}

	return corrID, nil
}

func GetComponentIDFromEvent(event Event, componentType ComponentType) (string, error) {
	if event == nil {
		return "", fmt.Errorf("event is nil")
	}

	switch componentType {
	case ComponentWorkflow:
		workflowInfo := event.GetWorkflow()
		if workflowInfo == nil {
			return "", fmt.Errorf("workflow info is nil")
		}
		return workflowInfo.GetId(), nil

	case ComponentTask:
		taskInfo := event.GetTask()
		if taskInfo == nil {
			return "", fmt.Errorf("task info is nil")
		}
		return taskInfo.GetId(), nil

	case ComponentAgent:
		agentInfo := event.GetAgent()
		if agentInfo == nil {
			return "", fmt.Errorf("agent info is nil")
		}
		return agentInfo.GetId(), nil

	case ComponentTool:
		toolInfo := event.GetTool()
		if toolInfo == nil {
			return "", fmt.Errorf("tool info is nil")
		}
		return toolInfo.GetId(), nil

	default:
		return "", fmt.Errorf("unsupported component type: %s", componentType)
	}
}

func GetComponentExecIDFromEvent(event Event, componentType ComponentType) (string, error) {
	if event == nil {
		return "", fmt.Errorf("event is nil")
	}

	switch componentType {
	case ComponentWorkflow:
		workflowInfo := event.GetWorkflow()
		if workflowInfo == nil {
			return "", fmt.Errorf("workflow info is nil")
		}
		return workflowInfo.GetExecId(), nil

	case ComponentTask:
		taskInfo := event.GetTask()
		if taskInfo == nil {
			return "", fmt.Errorf("task info is nil")
		}
		return taskInfo.GetExecId(), nil

	case ComponentAgent:
		agentInfo := event.GetAgent()
		if agentInfo == nil {
			return "", fmt.Errorf("agent info is nil")
		}
		return agentInfo.GetExecId(), nil

	case ComponentTool:
		toolInfo := event.GetTool()
		if toolInfo == nil {
			return "", fmt.Errorf("tool info is nil")
		}
		return toolInfo.GetExecId(), nil

	default:
		return "", fmt.Errorf("unsupported component type: %s", componentType)
	}
}

func ResultFromEvent(event Event) (*common.Result, error) {
	if event == nil {
		return nil, fmt.Errorf("event is nil")
	}

	payload := event.GetPayload()
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

	castedResult, ok := result.(*common.Result)
	if !ok {
		return nil, fmt.Errorf("result is not of type *common.Result")
	}

	return castedResult, nil
}

func StatusFromEvent(event Event) (EvStatusType, error) {
	if event == nil {
		return "", fmt.Errorf("event is nil")
	}

	payload := event.GetPayload()
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
