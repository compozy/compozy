package nats

import (
	"github.com/compozy/compozy/pkg/pb/common"
	"google.golang.org/protobuf/types/known/structpb"
)

// EventData provides a concrete implementation of the Event interface
// for handling event data received from NATS messages.
type EventData struct {
	// Subject is the NATS subject that the event was published on
	Subject string

	// Data is the raw message payload
	Data []byte

	// Type is the event type extracted from the subject
	Type string

	// Component-related fields
	Metadata *common.Metadata
	Workflow *common.WorkflowInfo
	Task     *common.TaskInfo
	Agent    *common.AgentInfo
	Tool     *common.ToolInfo

	// EventCtx contains the context data associated with the event
	EventCtx *structpb.Struct
}

// GetMetadata returns the event metadata
func (e *EventData) GetMetadata() *common.Metadata {
	return e.Metadata
}

// GetWorkflow returns workflow information
func (e *EventData) GetWorkflow() *common.WorkflowInfo {
	return e.Workflow
}

// GetTask returns task information
func (e *EventData) GetTask() *common.TaskInfo {
	return e.Task
}

// GetAgent returns agent information
func (e *EventData) GetAgent() *common.AgentInfo {
	return e.Agent
}

// GetTool returns tool information
func (e *EventData) GetTool() *common.ToolInfo {
	return e.Tool
}

// GetPayload returns a simple event payload implementation
func (e *EventData) GetPayload() EventPayload {
	return &EventPayloadData{
		Context: e.EventCtx,
	}
}

// EventPayloadData implements the EventPayload interface
// to provide access to the event's context data.
type EventPayloadData struct {
	Context *structpb.Struct
}

// GetContext returns the context from the payload
func (p *EventPayloadData) GetContext() *structpb.Struct {
	return p.Context
}

// NewEventData creates a new EventData instance with the basic required fields.
// Additional fields can be set directly on the returned instance if needed.
func NewEventData(subject string, data []byte, eventType string) *EventData {
	return &EventData{
		Subject: subject,
		Data:    data,
		Type:    eventType,
	}
}
