package project

import (
	"github.com/compozy/compozy/internal/parser/pkgref"
)

type ProjectName string
type ProjectVersion string
type ProjectDescription string
type LogLevel string
type EnvFilePath string
type Dependencies []*pkgref.PackageRef
type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentProduction  Environment = "production"
	EnvironmentStaging     Environment = "staging"

	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
)

// IsValidLogLevel checks if the given log level is valid
func IsValidLogLevel(level LogLevel) bool {
	switch level {
	case LogLevelDebug, LogLevelInfo, LogLevelWarning, LogLevelError:
		return true
	default:
		return false
	}
}
