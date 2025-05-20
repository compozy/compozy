package nats

import (
	"fmt"
	"strings"
)

const (
	SubjectPrefix   = "compozy"
	CommandsSegment = "cmds"
	EventsSegment   = "events"
)

// ParseEvtSubject parses a NATS state event subject into its component parts.
//
// Subject pattern: compozy.<correlation_id>.<component_type>.events.<component_id>.<event_type>
//
// Example subject: "compozy.abc123.workflow.events.def456.started"
// This would return:
//   - ComponentType: "workflow"
//   - componentID: "def456"
//   - corrID: "abc123"
//   - eventType: "started"
//
// Returns an error if the subject doesn't follow the expected format.
func ParseEvtSubject(subject string) (ComponentType, string, string, string, error) {
	parts := strings.Split(subject, ".")
	if len(parts) < 6 {
		return "", "", "", "", fmt.Errorf("invalid state event subject format: %s, expected at least 6 parts", subject)
	}

	// Validate prefix
	if parts[0] != SubjectPrefix {
		return "", "", "", "", fmt.Errorf("invalid subject prefix: %s, expected 'compozy'", parts[0])
	}

	// Extract components
	corrID := parts[1]
	componentType := ComponentType(parts[2])

	// Validate events segment
	if parts[3] != EventsSegment {
		return "", "", "", "", fmt.Errorf("invalid segment type: %s, expected 'events'", parts[3])
	}

	componentID := parts[4]
	eventType := parts[5]

	// Validate component type
	switch componentType {
	case ComponentWorkflow, ComponentTask, ComponentAgent, ComponentTool:
		// Valid component type
	default:
		return "", "", "", "", fmt.Errorf(
			"invalid component type: %s, expected one of workflow, task, agent, or tool",
			componentType,
		)
	}

	return componentType, componentID, corrID, eventType, nil
}

// BuildEvtSubject builds a NATS subject for state events.
//
// Pattern: compozy.<correlation_id>.<component_type>.events.<component_id>.<event_type>
//
// Example usage:
//
//	subject := BuildEvtSubject(ComponentWorkflow, "workflow-123", "corr-456", "started")
//	// Results in: "compozy.corr-456.workflow.events.workflow-123.started"
func BuildEvtSubject(componentType ComponentType, componentID, corrID, eventType string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s.%s",
		SubjectPrefix,
		corrID,
		componentType,
		EventsSegment,
		componentID,
		eventType)
}
