package log

import (
	"fmt"

	"github.com/compozy/compozy/pkg/pb"
)

func logLevelToStr(logLevel LogLevel) string {
	switch logLevel {
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
// Pattern: compozy.logs.<correlation_id>.<component>.<log_level>
func (x *LogEmittedEvent) ToSubject() string {
	correlationID := pb.GetCorrelationID(x)
	component := pb.GetSourceComponent(x)
	logLevelStr := logLevelToStr(x.GetPayload().GetLogLevel())
	return fmt.Sprintf("compozy.logs.%s.%s.%s", correlationID, component, logLevelStr)
}
