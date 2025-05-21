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

// getComponentInfoFromEvent retrieves the component info and ID/ExecID from the event based on component type
// idType can be "id" or "exec_id" to determine which ID to retrieve
func getComponentInfoFromEvent(evt Event, compType ComponentType, idType string) (string, error) {
	if evt == nil {
		return "", fmt.Errorf("event is nil")
	}

	var info any
	var componentName string

	switch compType {
	case ComponentWorkflow:
		info = evt.GetWorkflow()
		componentName = "workflow"
	case ComponentTask:
		info = evt.GetTask()
		componentName = "task"
	case ComponentAgent:
		info = evt.GetAgent()
		componentName = "agent"
	case ComponentTool:
		info = evt.GetTool()
		componentName = "tool"
	default:
		return "", fmt.Errorf("unsupported component type: %s", compType)
	}

	if info == nil {
		return "", fmt.Errorf("%s info is nil", componentName)
	}

	infoValue := reflect.ValueOf(info)
	var methodName string

	switch idType {
	case "id":
		methodName = "GetId"
	case "exec_id":
		methodName = "GetExecId"
	default:
		return "", fmt.Errorf("unsupported ID type: %s", idType)
	}

	method := infoValue.MethodByName(methodName)
	if !method.IsValid() {
		return "", fmt.Errorf("%s method not found for %s", methodName, compType)
	}

	result := method.Call(nil)
	if len(result) == 0 {
		return "", fmt.Errorf("%s method returned no value", methodName)
	}

	idValue := result[0].Interface()
	idStr, ok := idValue.(string)
	if !ok {
		return "", fmt.Errorf("failed to convert %s to string", methodName)
	}

	return idStr, nil
}

func GetCompIDFromEvent(evt Event, compType ComponentType) (common.CompID, error) {
	id, err := getComponentInfoFromEvent(evt, compType, "id")
	if err != nil {
		return "", err
	}
	return common.CompID(id), nil
}

func GetExecIDFromEvent(evt Event, compType ComponentType) (common.ExecID, error) {
	id, err := getComponentInfoFromEvent(evt, compType, "exec_id")
	if err != nil {
		return "", err
	}
	return common.ExecID(id), nil
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
