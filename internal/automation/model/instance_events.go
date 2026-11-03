package model

import (
	"context"
	"errors"
)

var ErrTriggerTargetInstanceNotFound = errors.New("trigger_target_instance_not_found")

type InstanceTarget struct {
	ProfileID       string
	WorkspaceID     string
	AgentName       string
	ChecksSupported bool
}

type InstanceEvent struct {
	TriggerName, Event, Instruction string
	RunID                           string
	TriggerID                       string
	Summary                         string
}

type InstanceEventDelivery struct {
	InstanceID     string
	EventID        string
	CheckScheduled bool
	Reason         string
}

type InstanceEventSink interface {
	Deliver(context.Context, InstanceTarget, InstanceEvent) (InstanceEventDelivery, error)
}
