package pb

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

func logLevelToStr(lvl LogLevel) logger.LogLevel {
	switch lvl {
	case LogLevel_LOG_LEVEL_DEBUG:
		return logger.DebugLevel
	case LogLevel_LOG_LEVEL_INFO:
		return logger.InfoLevel
	case LogLevel_LOG_LEVEL_WARN:
		return logger.WarnLevel
	case LogLevel_LOG_LEVEL_ERROR:
		return logger.ErrorLevel
	default:
		return logger.NoLevel
	}
}

// ToSubject generates the NATS subject for a EventLogEmitted.
// Pattern: compozy.<workflow_exec_id>.<component>.logs.<component_exec_id>.<log_level>
func (x *EventLogEmitted) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	comp := core.ComponentType(x.GetDetails().GetComponent())
	execID := x.GetDetails().GetComponentExecId()
	lvl := logLevelToStr(x.GetDetails().GetLogLevel())
	return core.BuildLogSubject(comp, wExecID, execID, lvl)
}

func (x *EventLogEmitted) ToSubjectParams(workflowExecID string, execID string) string {
	comp := core.ComponentType(x.GetDetails().GetComponent())
	lvl := logLevelToStr(x.GetDetails().GetLogLevel())
	return core.BuildLogSubject(comp, workflowExecID, execID, lvl)
}
