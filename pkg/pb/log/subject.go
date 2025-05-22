package log

import (
	"fmt"

	"github.com/compozy/compozy/pkg/pb"
)

func logLevelToStr(lvl LogLevel) string {
	switch lvl {
	case LogLevel_LOG_LEVEL_DEBUG:
		return "debug"
	case LogLevel_LOG_LEVEL_INFO:
		return "info"
	case LogLevel_LOG_LEVEL_WARN:
		return "warn"
	case LogLevel_LOG_LEVEL_ERROR:
		return "error"
	default:
		return "unspecified"
	}
}

// ToSubject generates the NATS subject for a LogEmittedEvent.
// Pattern: compozy.<correlation_id>.<component>.logs.<component_id>.<log_level>
func (x *LogEmittedEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	comp := x.GetDetails().GetComponent()
	compID := x.GetDetails().GetComponentId()
	lvl := logLevelToStr(x.GetDetails().GetLogLevel())
	return fmt.Sprintf("compozy.%s.%s.logs.%s.%s", corrID, comp, compID, lvl)
}
