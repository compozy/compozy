package nats

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	SubjectPrefix   = "compozy"
	MinSubjectParts = 6
)

type SubjectSegment string

const (
	SegmentCmd   SubjectSegment = "cmds"
	SegmentEvent SubjectSegment = "evts"
	SegmentLog   SubjectSegment = "logs"
)

type SubjectData struct {
	CompType ComponentType
	ExecID   common.ExecID
	CorrID   common.CorrID
}

type EventSubject struct {
	SubjectData
	EventType EvtType
}

type CommandSubject struct {
	SubjectData
	CommandType CmdType
}

type LogSubject struct {
	SubjectData
	LogLevel logger.LogLevel
}

var logLevelMap = map[string]logger.LogLevel{
	"debug": logger.DebugLevel,
	"info":  logger.InfoLevel,
	"warn":  logger.WarnLevel,
	"error": logger.ErrorLevel,
}

func parseSubject(subject string, expectedSegment SubjectSegment) (SubjectData, string, error) {
	parts := strings.Split(subject, ".")
	if len(parts) < MinSubjectParts {
		return SubjectData{}, "", fmt.Errorf("invalid subject format: %s, expected at least %d parts",
			subject, MinSubjectParts)
	}

	if parts[0] != SubjectPrefix {
		return SubjectData{}, "", fmt.Errorf("invalid subject prefix: %s, expected %q",
			parts[0], SubjectPrefix)
	}

	if parts[3] != string(expectedSegment) {
		return SubjectData{}, "", fmt.Errorf("invalid segment type: %s, expected %q",
			parts[3], expectedSegment)
	}

	compType := ComponentType(parts[2])
	return SubjectData{
		CompType: compType,
		CorrID:   common.CorrID(parts[1]),
		ExecID:   common.ExecID(parts[4]),
	}, parts[5], nil
}

func ParseEvtSubject(subject string) (*EventSubject, error) {
	data, val, err := parseSubject(subject, SegmentEvent)
	if err != nil {
		return nil, err
	}
	return &EventSubject{
		SubjectData: data,
		EventType:   EvtType(val),
	}, nil
}

func ParseCmdSubject(subject string) (*CommandSubject, error) {
	data, val, err := parseSubject(subject, SegmentCmd)
	if err != nil {
		return nil, err
	}
	return &CommandSubject{
		SubjectData: data,
		CommandType: CmdType(val),
	}, nil
}

func ParseLogSubject(subject string) (*LogSubject, error) {
	data, val, err := parseSubject(subject, SegmentLog)
	if err != nil {
		return nil, err
	}

	lvl, ok := logLevelMap[strings.ToLower(val)]
	if !ok {
		lvl = logger.InfoLevel // Default to InfoLevel for unrecognized levels
	}

	return &LogSubject{
		SubjectData: data,
		LogLevel:    lvl,
	}, nil
}

func BuildEvtSubject(comp ComponentType, corrID common.CorrID, execID common.ExecID, evt EvtType) string {
	return buildSubject(comp, corrID, execID, SegmentEvent, evt.String())
}

func BuildCmdSubject(comp ComponentType, corrID common.CorrID, execID common.ExecID, cmd CmdType) string {
	return buildSubject(comp, corrID, execID, SegmentCmd, cmd.String())
}

func BuildLogSubject(comp ComponentType, corrID common.CorrID, execID common.ExecID, log logger.LogLevel) string {
	return buildSubject(comp, corrID, execID, SegmentLog, log.String())
}

func buildSubject(
	comp ComponentType,
	corrID common.CorrID,
	execID common.ExecID,
	segment SubjectSegment,
	val string,
) string {
	return strings.Join([]string{
		SubjectPrefix,
		corrID.String(),
		string(comp),
		string(segment),
		execID.String(),
		val,
	}, ".")
}
