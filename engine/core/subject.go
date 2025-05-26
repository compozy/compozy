package core

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/pkg/logger"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Subjecter interface {
	protoreflect.ProtoMessage
	ToSubject() string
	ToSubjectParams(workflowExecID string, execID string) string
}

type SubjectData struct {
	Component       ComponentType
	WorkflowExecID  ID
	ComponentExecID ID
	Segment         SubjectSegmentType
	SegmentAction   string
}

type EventSubject struct {
	*SubjectData
	EventType EvtType
}

type CommandSubject struct {
	*SubjectData
	CommandType CmdType
}

type LogSubject struct {
	*SubjectData
	LogLevel logger.LogLevel
}

var logLevelMap = map[string]logger.LogLevel{
	"debug": logger.DebugLevel,
	"info":  logger.InfoLevel,
	"warn":  logger.WarnLevel,
	"error": logger.ErrorLevel,
}

func parseSubject(subject string, segment SubjectSegmentType) (*SubjectData, error) {
	parts := strings.Split(subject, ".")
	if len(parts) < MinSubjectParts {
		return nil, fmt.Errorf("invalid subject format: %s, expected at least %d parts", subject, MinSubjectParts)
	}
	if parts[0] != SubjectPrefix {
		return nil, fmt.Errorf("invalid subject prefix: %s, expected %q", parts[0], SubjectPrefix)
	}
	if parts[3] != string(segment) {
		return nil, fmt.Errorf("invalid segment type: %s, expected %q", parts[3], segment)
	}
	return &SubjectData{
		Component:       ComponentType(parts[2]),
		WorkflowExecID:  ID(parts[1]),
		ComponentExecID: ID(parts[4]),
		Segment:         segment,
		SegmentAction:   parts[5],
	}, nil
}

func ParseEvtSubject(subject string) (*EventSubject, error) {
	data, err := parseSubject(subject, SegmentEvent)
	if err != nil {
		return nil, err
	}
	return &EventSubject{
		SubjectData: data,
		EventType:   EvtType(data.SegmentAction),
	}, nil
}

func ParseCmdSubject(subject string) (*CommandSubject, error) {
	data, err := parseSubject(subject, SegmentCmd)
	if err != nil {
		return nil, err
	}
	return &CommandSubject{
		SubjectData: data,
		CommandType: CmdType(data.SegmentAction),
	}, nil
}

func ParseLogSubject(subject string) (*LogSubject, error) {
	data, err := parseSubject(subject, SegmentLog)
	if err != nil {
		return nil, err
	}
	lvl, ok := logLevelMap[strings.ToLower(data.SegmentAction)]
	if !ok {
		lvl = logger.InfoLevel // Default to InfoLevel for unrecognized levels
	}
	return &LogSubject{
		SubjectData: data,
		LogLevel:    lvl,
	}, nil
}

func BuildEvtSubject(comp ComponentType, workflowExecID string, execID string, evt EvtType) string {
	return buildSubject(comp, workflowExecID, execID, SegmentEvent, evt.String())
}

func BuildCmdSubject(comp ComponentType, workflowExecID string, execID string, cmd CmdType) string {
	return buildSubject(comp, workflowExecID, execID, SegmentCmd, cmd.String())
}

func BuildLogSubject(comp ComponentType, workflowExecID string, execID string, lvl logger.LogLevel) string {
	return buildSubject(comp, workflowExecID, execID, SegmentLog, lvl.String())
}

func buildSubject(
	comp ComponentType,
	workflowExecID string,
	execID string,
	segment SubjectSegmentType,
	val string,
) string {
	return strings.Join([]string{
		SubjectPrefix,
		workflowExecID,
		string(comp),
		string(segment),
		execID,
		val,
	}, ".")
}
